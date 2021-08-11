package proxy

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/kubectl/pkg/util/podutils"
)

var GenerateNameSuffix = regexp.MustCompile("-[0-9a-f]+-$")

type RequestIDGenerator struct {
	requestIDLock sync.Mutex
	requestID     int
}

func (r *RequestIDGenerator) nextRequestID() int {
	r.requestIDLock.Lock()
	defer r.requestIDLock.Unlock()
	id := r.requestID
	r.requestID++
	return id
}

type PortForwardConnection struct {
	RequestIDGenerator
	httpstream.Connection
	wg sync.WaitGroup
}

// Forward copies data between the local connection and the stream to
// the remote server.
// see https://github.com/kubernetes/client-go/blob/master/tools/portforward/portforward.go#L324
// see https://github.com/kubernetes/kubernetes/blob/10ed4502f46d763a809ccdcc6c30be1c03e19147/pkg/kubelet/cri/streaming/server.go#L132
// see https://github.com/kubernetes/kubernetes/blob/10ed4502f46d763a809ccdcc6c30be1c03e19147/pkg/kubelet/cri/streaming/portforward/portforward.go#L41
// see https://github.com/kubernetes/kubernetes/blob/10ed4502f46d763a809ccdcc6c30be1c03e19147/pkg/kubelet/cri/streaming/portforward/httpstream.go#L36
// see https://github.com/kubernetes/kubernetes/blob/10ed4502f46d763a809ccdcc6c30be1c03e19147/pkg/kubelet/cri/streaming/portforward/httpstream.go#L74
func (forwarder *PortForwardConnection) Forward(conn net.Conn, port uint16) error {
	forwarder.wg.Add(1)
	defer forwarder.wg.Done()
	defer conn.Close()
	requestID := forwarder.nextRequestID()

	// create error stream
	headers := http.Header{}
	headers.Set(corev1.StreamType, corev1.StreamTypeError)
	headers.Set(corev1.PortHeader, fmt.Sprintf("%d", port))
	headers.Set(corev1.PortForwardRequestIDHeader, strconv.Itoa(requestID))
	errorStream, err := forwarder.CreateStream(headers)
	if err != nil {
		return err
	}
	// we're not writing to this stream
	errorStream.Close()

	go func() {
		message, err := ioutil.ReadAll(errorStream)
		if err != nil {
			runtime.HandleError(fmt.Errorf("error reading from error stream for port %d: %v", port, err))
		}
		if len(message) > 0 {
			runtime.HandleError(fmt.Errorf("an error occurred forwarding %d: %v", port, string(message)))
		}
	}()

	// create data stream
	headers.Set(corev1.StreamType, corev1.StreamTypeData)
	dataStream, err := forwarder.CreateStream(headers)
	if err != nil {
		runtime.HandleError(fmt.Errorf("error creating forwarding stream for port %d: %v", port, err))
		return err
	}
	localError := make(chan struct{})
	remoteDone := make(chan struct{})

	go func() {
		// inform the select below that the remote copy is done
		defer close(remoteDone)

		// Copy from the remote side to the local port.
		_, err := io.Copy(conn, dataStream)
		if err == nil {
			return
		}
		if err == io.ErrClosedPipe {
			return
		}
		if strings.Contains(err.Error(), "use of closed network connection") {
			return
		}
	}()

	go func() {
		// inform server we're not sending any more data after copy unblocks
		defer close(localError)
		defer dataStream.Close()

		// Copy from the local port to the remote side.
		_, err := io.Copy(dataStream, conn)
		if err == nil {
			return
		}
		if err == io.ErrClosedPipe {
			return
		}
		if strings.Contains(err.Error(), "use of closed network connection") {
			return
		}
		runtime.HandleError(fmt.Errorf("error copying from local connection to remote stream: %v", err))
	}()

	// wait for either a local->remote error or for copying from remote->local to finish
	select {
	case <-remoteDone:
	case <-localError:
	}
	return nil
}

func (forwarder *PortForwardConnection) Close() error {
	forwarder.wg.Wait()
	return forwarder.Connection.Close()
}

func DialPortForwardConnection(client *rest.RESTClient, config *rest.Config, namespace string, podName string) (*PortForwardConnection, error) {
	var err error
	req := client.Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("portforward")

	method := http.MethodPost
	portforwardURL := req.URL()
	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return nil, err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, method, portforwardURL)
	streamConn, _, err := dialer.Dial(portforward.PortForwardProtocolV1Name)
	if err != nil {
		return nil, err
	}

	return &PortForwardConnection{
		Connection: streamConn,
	}, nil
}

type PodBackend struct {
	OnCreatePortForward func()
	OnClosePortForward  func()

	name       string
	dialOnce   sync.Once
	connection *PortForwardConnection
	err        error
}

func (backend *PodBackend) Close() error {
	if backend.connection == nil {
		return nil
	}
	return backend.connection.Close()
}

func (backend *PodBackend) Name() string {
	return backend.name
}

func (backend *PodBackend) DialPortForwardOnce(client *rest.RESTClient, config *rest.Config, namespace string) (*PortForwardConnection, error) {
	backend.dialOnce.Do(func() {
		connection, err := DialPortForwardConnection(client, config, namespace, backend.name)
		if err != nil {
			backend.err = err
			return
		}
		backend.connection = connection
		if backend.OnCreatePortForward != nil {
			go backend.OnCreatePortForward()
		}
		go func() {
			<-backend.connection.CloseChan()
			if backend.OnClosePortForward == nil {
				return
			}
			go backend.OnClosePortForward()
		}()
	})

	if backend.err != nil {
		return nil, backend.err
	}
	return backend.connection, nil
}

func NewPodBackend(name string) *PodBackend {
	return &PodBackend{
		name: name,
	}
}

type ServiceDirector struct {
	name               string
	targetPort         uint16
	activePodMapLock   sync.RWMutex
	activePodMap       map[string]*PodBackend
	directorPodMapLock sync.RWMutex
	directorPodMap     map[string]*PodBackend
}

func (director *ServiceDirector) Evict(podName string) {
	director.activePodMapLock.Lock()
	defer director.activePodMapLock.Unlock()
	backend := director.activePodMap[podName]
	if backend == nil {
		return
	}
	delete(director.activePodMap, podName)

	director.directorPodMapLock.Lock()
	defer director.directorPodMapLock.Unlock()
	delete(director.directorPodMap, podName)
	go backend.Close()
}

func (director *ServiceDirector) UpdatePodMap(pod *corev1.Pod) *PodBackend {
	ready := podutils.IsPodReady(pod)
	if !ready {
		director.Evict(pod.Name)
		return nil
	}
	director.activePodMapLock.Lock()
	defer director.activePodMapLock.Unlock()
	pb := director.activePodMap[pod.Name]
	if pb != nil {
		return nil
	}

	pb = NewPodBackend(pod.Name)
	director.activePodMap[pod.Name] = pb
	return pb
}

func (director *ServiceDirector) LookupPodBackend() *PodBackend {
	director.directorPodMapLock.Lock()
	defer director.directorPodMapLock.Unlock()
	for _, pod := range director.directorPodMap {
		return pod
	}

	director.activePodMapLock.RLock()
	defer director.activePodMapLock.RUnlock()
	for podName, pod := range director.activePodMap {
		director.directorPodMap[podName] = pod
		return pod
	}
	return nil
}

func (director *ServiceDirector) TargetPort() uint16 {
	return director.targetPort
}

func NewServiceDirector(name string, targetPort uint16) *ServiceDirector {
	return &ServiceDirector{
		name:           name,
		targetPort:     targetPort,
		activePodMap:   map[string]*PodBackend{},
		directorPodMap: map[string]*PodBackend{},
	}
}

type HttpPortForwardResolver struct {
	serviceMapLock  sync.RWMutex
	serviceMap      map[string]*ServiceDirector
	OnAddPodBackend func(backend *PodBackend)
}

func (resolver *HttpPortForwardResolver) UpdatePodBackend(pod *corev1.Pod) {
	resolver.serviceMapLock.RLock()
	defer resolver.serviceMapLock.RUnlock()
	svcName := GenerateNameSuffix.ReplaceAllString(pod.GenerateName, "")
	svc := resolver.serviceMap[svcName]
	if svc == nil {
		return
	}
	backend := svc.UpdatePodMap(pod)
	if backend == nil {
		return
	}
	if resolver.OnAddPodBackend == nil {
		return
	}
	go resolver.OnAddPodBackend(backend)
}

func (resolver *HttpPortForwardResolver) AddServiceMap(svc *corev1.Service, targetPort uint16) *ServiceDirector {
	name := svc.ObjectMeta.Name
	resolver.serviceMapLock.Lock()
	defer resolver.serviceMapLock.Unlock()
	_, exists := resolver.serviceMap[name]
	if exists {
		return nil
	}
	svcDirector := NewServiceDirector(
		name,
		targetPort,
	)

	resolver.serviceMap[name] = svcDirector
	return svcDirector
}

func (resolver *HttpPortForwardResolver) GetServiceNames() []string {
	items := []string{}
	for name, _ := range resolver.serviceMap {
		items = append(items, name)
	}
	return items
}

func (resolver *HttpPortForwardResolver) LookupServiceDirector(svcName string) *ServiceDirector {
	resolver.serviceMapLock.RLock()
	defer resolver.serviceMapLock.RUnlock()
	return resolver.serviceMap[svcName]
}

func (resolver *HttpPortForwardResolver) LookupPodBackend(svcName string) *PodBackend {
	resolver.serviceMapLock.RLock()
	defer resolver.serviceMapLock.RUnlock()
	svc := resolver.serviceMap[svcName]
	if svc == nil {
		return nil
	}
	return svc.LookupPodBackend()
}

func (resolver *HttpPortForwardResolver) GetHttpTransport(client *rest.RESTClient, config *rest.Config, namespace string) *http.Transport {
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}

			director := resolver.LookupServiceDirector(host)
			if director == nil {
				// TODO http 503
				err = fmt.Errorf("%s svc not found", host)
				runtime.HandleError(err)
				return nil, err
			}

			backend := director.LookupPodBackend()
			if backend == nil {
				err := fmt.Errorf("%s pod not found", director.name)
				return nil, err
			}

			conn, err := backend.DialPortForwardOnce(client, config, namespace)
			if err != nil {
				director.Evict(backend.Name())
				return nil, err
			}

			local, remote := net.Pipe()
			go func() {
				defer local.Close()
				err := conn.Forward(remote, director.TargetPort())
				if err != nil {
					director.Evict(backend.Name())
				}
			}()

			return local, nil
		},
	}
}

func NewPortForwardResolver() *HttpPortForwardResolver {
	resolver := &HttpPortForwardResolver{
		serviceMap: map[string]*ServiceDirector{},
	}
	return resolver
}
