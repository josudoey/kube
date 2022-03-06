package vhost

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"k8s.io/kubectl/pkg/util/podutils"
)

type RequestIDGenerator struct {
	requestIDLock sync.Mutex
	requestID     int
}

func (r *RequestIDGenerator) NextRequestID() int {
	r.requestIDLock.Lock()
	defer r.requestIDLock.Unlock()
	id := r.requestID
	r.requestID++
	return id
}

type PortForwardConnection struct {
	OnCreateStream func(id int)
	OnCloseStream  func(id int)

	RequestIDGenerator
	httpstream.Connection
	wg sync.WaitGroup
}

// Forward copies data between the local connection and the stream to
// the remote server.
// see https://github.com/kubernetes/client-go/blob/94daee0164805ef86cc36790c662b7f074db10ec/tools/portforward/portforward.go#L324
// see https://github.com/kubernetes/kubernetes/blob/10ed4502f46d763a809ccdcc6c30be1c03e19147/pkg/kubelet/cri/streaming/server.go#L132
// see https://github.com/kubernetes/kubernetes/blob/10ed4502f46d763a809ccdcc6c30be1c03e19147/pkg/kubelet/cri/streaming/portforward/portforward.go#L41
// see https://github.com/kubernetes/kubernetes/blob/10ed4502f46d763a809ccdcc6c30be1c03e19147/pkg/kubelet/cri/streaming/portforward/httpstream.go#L36
// see https://github.com/kubernetes/kubernetes/blob/10ed4502f46d763a809ccdcc6c30be1c03e19147/pkg/kubelet/cri/streaming/portforward/httpstream.go#L74
func (forwarder *PortForwardConnection) Forward(conn net.Conn, port uint16, clientPreface []byte) error {
	forwarder.wg.Add(1)
	defer forwarder.wg.Done()
	defer conn.Close()
	requestID := forwarder.NextRequestID()

	// create error stream
	headers := http.Header{}
	headers.Set(v1.StreamType, v1.StreamTypeError)
	headers.Set(v1.PortHeader, fmt.Sprintf("%d", port))
	headers.Set(v1.PortForwardRequestIDHeader, strconv.Itoa(requestID))
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
	headers.Set(v1.StreamType, v1.StreamTypeData)
	dataStream, err := forwarder.CreateStream(headers)
	if err != nil {
		runtime.HandleError(fmt.Errorf("error creating forwarding stream for port %d: %v", port, err))
		return err
	}

	if forwarder.OnCreateStream != nil {
		go forwarder.OnCreateStream(requestID)
	}
	localError := make(chan struct{})
	remoteDone := make(chan struct{})

	dataStream.Write(clientPreface)
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
		runtime.HandleError(fmt.Errorf("error copying from remote stream to local connection: %v", err))
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

	if forwarder.OnCloseStream != nil {
		go forwarder.OnCloseStream(requestID)
	}
	return nil
}

func (forwarder *PortForwardConnection) Close() error {
	forwarder.wg.Wait()
	return forwarder.Connection.Close()
}

func DialPortForwardConnection(client rest.Interface, config *rest.Config, namespace string, podName string) (*PortForwardConnection, error) {
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

func (resolver *PortForwardResolver) AddService(svc v1.Service) []*ServicePortEntry {
	_, selector, err := polymorphichelpers.SelectorsForObject(&svc)
	if err != nil {
		return nil
	}

	for _, svcPort := range svc.Spec.Ports {
		port := uint16(svcPort.TargetPort.IntVal)
		if port == 0 {
			port = uint16(svcPort.Port)
		}

		if port == 0 {
			continue
		}

		entry := &ServicePortEntry{
			Service:     svc,
			ServicePort: svcPort,
			Selector:    selector,
		}

		resolver.router.AddIfNotExists(entry)
	}
	return nil
}

func (resolver *PortForwardResolver) AddPod(pod v1.Pod) {
	podName := pod.GetName()
	ready := podutils.IsPodReady(&pod)
	if !ready {
		return
	}

	_, exists := resolver.pods.GetOrSet(podName, &pod)
	if exists {
		return
	}

	for _, service := range resolver.router.Values() {
		if !service.Match(pod) {
			continue
		}

		matchedPod := &MatchedPod{
			ServicePort: service.ServicePort,
			Pod:         pod,
		}

		backend := NewPodBackend(matchedPod)
		resolver.activeBackend.Add(service, backend)
		if resolver.OnAddServiceBackend == nil {
			continue
		}
		go resolver.OnAddServiceBackend(*service, backend)
	}
}

func (resolver *PortForwardResolver) DeleteByName(podName string) {
	resolver.pods.Delete(podName)
	resolver.activeBackend.Range(func(key *ServicePortEntry, value *PodBackendSet) bool {
		value.DeleteByName(podName)
		return true
	})
}

func (resolver *PortForwardResolver) UpdatePod(pod *v1.Pod) {
	podName := pod.GetName()
	ready := podutils.IsPodReady(pod)
	_, exists := resolver.pods.Get(podName)

	if exists == ready {
		return
	}

	if !ready {
		resolver.DeleteByName(podName)
		return
	}

	resolver.AddPod(*pod)
}

func (resolver *PortForwardResolver) ResolveBackend(hostname string) *PodBackend {
	entry := resolver.router.Resolve(hostname)
	if entry == nil {
		return nil
	}

	return resolver.activeBackend.GetOne(entry)
}

func (resolver *PortForwardResolver) ResolveAddr(addr string) string {
	entry := resolver.router.Resolve(addr)
	if entry == nil {
		return addr
	}
	backend := resolver.activeBackend.GetOne(entry)
	if backend == nil {
		return entry.TargetHostPort()
	}
	return backend.GetTargetHostPort()
}

func (resolver *PortForwardResolver) ListServices() []ServicePortEntry {
	items := []ServicePortEntry{}
	for _, item := range resolver.router.Values() {
		items = append(items, *item)
	}
	return items
}

type PortForwardResolver struct {
	router        ServicePortEntryRouter
	pods          PodMap
	activeBackend ServiceBackend

	OnAddServiceBackend func(entry ServicePortEntry, backend *PodBackend)
}

func NewPortForwardResolver() *PortForwardResolver {
	resolver := &PortForwardResolver{}
	return resolver
}
