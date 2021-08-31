package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/josudoey/kube"
	"github.com/josudoey/kube/vhost"
	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

const (
	defaultPort    = 8010
	defaultAddress = "127.0.0.1"
)

type KubeVhostOptions struct {
	port    int
	address string
	verbose bool

	LabelSelector string
}

func NewKubeVhostOptions() *KubeVhostOptions {
	return &KubeVhostOptions{
		port:    defaultPort,
		address: defaultAddress,
	}
}

func (o *KubeVhostOptions) Run(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	selector := o.LabelSelector
	namespace, _, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	client, err := kube.GetClient(f)
	if err != nil {
		return err
	}

	ctx := context.Background()
	serviceList, err := kube.GetServiceList(ctx, client, namespace, selector)
	if err != nil {
		return err
	}
	httpResolver := vhost.NewPortForwardResolver()
	grpcResolver := vhost.NewPortForwardResolver()
	for _, svc := range serviceList.Items {
		for _, svcPort := range kube.GetServicePorts(&svc) {
			if svcPort.Name == "http" || svcPort.Port == 80 {
				targetPort := uint16(svcPort.TargetPort.IntVal)
				httpResolver.AddServiceMap(&svc, targetPort)
				continue
			}

			if svcPort.Name == "grpc" || svcPort.Port == 81 {
				targetPort := uint16(svcPort.TargetPort.IntVal)
				grpcResolver.AddServiceMap(&svc, targetPort)
				continue
			}
		}
	}

	httpResolver.OnAddPodBackend = func(backend *vhost.PodBackend) {
		podName := backend.Name()
		backend.OnCreatePortForward = func() {
			log.Printf("Created http PortForward %s", podName)
		}
		backend.OnClosePortForward = func() {
			log.Printf("Closed http PortForward %s", podName)
		}
		backend.OnCreateStream = func(id int) {
			log.Printf("Created http Stream#%d %s", id, podName)
		}
		backend.OnCloseStream = func(id int) {
			log.Printf("Closed http Stream#%d %s", id, podName)
		}
	}

	grpcResolver.OnAddPodBackend = func(backend *vhost.PodBackend) {
		podName := backend.Name()
		backend.OnCreatePortForward = func() {
			log.Printf("Created grpc PortForward %s", podName)
		}
		backend.OnClosePortForward = func() {
			log.Printf("Closed grpc PortForward %s", podName)
		}
		backend.OnCreateStream = func(id int) {
			log.Printf("Created grpc Stream#%d %s", id, podName)
		}
		backend.OnCloseStream = func(id int) {
			log.Printf("Closed grpc Stream#%d %s", id, podName)
		}
	}

	podList, err := kube.GetPodList(ctx, client, namespace, selector)
	for _, pod := range podList.Items {
		httpResolver.UpdatePodBackend(&pod)
		grpcResolver.UpdatePodBackend(&pod)
	}
	if err != nil {
		return err
	}

	watcher, err := kube.GetPodWatcher(ctx, client, namespace, selector, podList.ResourceVersion)
	if err != nil {
		return err
	}
	defer watcher.Stop()

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		ch := watcher.ResultChan()
		for {
			select {
			case e, ok := <-ch:
				if !ok {
					log.Printf("PodWatcher Closed")
					cancel()
					return
				}

				pod := kube.GetPod(e.Object)
				if pod == nil {
					continue
				}

				if o.verbose {
					ready := ""
					if kube.IsPodReady(pod) {
						ready = "(Ready)"
					}
					log.Printf("Event: %s %s%v %s", e.Type, pod.Status.Phase, ready, pod.Name)
				}
				httpResolver.UpdatePodBackend(pod)
				grpcResolver.UpdatePodBackend(pod)
			}
		}
	}()

	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	restClient, err := f.RESTClient()
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	transport := httpResolver.GetHttpTransport(restClient, config, namespace)
	for _, svc := range httpResolver.GetServiceDirectors() {
		name := svc.Name()
		vhost, err := url.Parse("http://" + name)
		if err != nil {
			continue
		}
		rp := httputil.NewSingleHostReverseProxy(vhost)
		rp.Transport = transport
		pattern := name + "/"
		mux.HandleFunc(pattern, rp.ServeHTTP)
		log.Printf("vhost http proxy %s:%d", name, svc.TargetPort())
	}

	for _, svc := range grpcResolver.GetServiceDirectors() {
		name := svc.Name()
		log.Printf("vhost grpc proxy %s:%d", name, svc.TargetPort())
	}

	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", o.address, o.port))
	if err != nil {
		return err
	}
	log.Printf("Starting to serve on %s\n", l.Addr().String())

	server := &http.Server{
		Handler: grpcResolver.GetGRPCHandler(mux, restClient, config, namespace),
	}
	go server.Serve(l)
	<-ctx.Done()
	server.Close()
	return nil
}

func main() {
	o := NewKubeVhostOptions()
	f := kube.GetDefaultFactory()

	cmd := &cobra.Command{
		Use: "kube-vhost [--port=PORT]",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Run(f, cmd, args))
		},
	}

	cmd.Flags().BoolVarP(&o.verbose, "verbose", "v", o.verbose, "Set verbose mode.")
	cmd.Flags().IntVarP(&o.port, "port", "p", o.port, "The port on which to run the proxy. Set to 0 to pick a random port.")
	cmd.Flags().StringVar(&o.address, "address", o.address, "The IP address on which to serve on.")
	cmd.Flags().StringVarP(&o.LabelSelector, "selector", "l", o.LabelSelector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	cmd.Execute()
}
