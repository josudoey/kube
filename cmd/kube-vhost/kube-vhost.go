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

type VhostProxyOptions struct {
	port    int
	address string
	verbose bool

	LabelSelector string
}

func NewVhostProxyOptions() *VhostProxyOptions {
	return &VhostProxyOptions{
		port:    defaultPort,
		address: defaultAddress,
	}
}

func (o *VhostProxyOptions) Run(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
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
	resolver := vhost.NewPortForwardResolver()
	for _, svc := range serviceList.Items {
		for _, svcPort := range kube.GetServicePorts(&svc) {
			if svcPort.Name != "http" && svcPort.Port != 80 {
				continue
			}

			targetPort := uint16(svcPort.TargetPort.IntVal)
			resolver.AddServiceMap(&svc, targetPort)
			break
		}
	}

	resolver.OnAddPodBackend = func(backend *vhost.PodBackend) {
		podName := backend.Name()
		backend.OnCreatePortForward = func() {
			log.Printf("Created PortForward %s", podName)
		}
		backend.OnClosePortForward = func() {
			log.Printf("Close PortForward %s", podName)
		}
	}

	podList, err := kube.GetPodList(ctx, client, namespace, selector)
	for _, pod := range podList.Items {
		resolver.UpdatePodBackend(&pod)
	}

	watcher, err := kube.GetPodWatcher(ctx, client, namespace, selector, podList)
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
				resolver.UpdatePodBackend(pod)
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
	transport := resolver.GetHttpTransport(restClient, config, namespace)
	for _, svc := range resolver.GetServiceDirectors() {
		name := svc.Name()
		vhost, err := url.Parse("http://" + name)
		if err != nil {
			continue
		}
		rp := httputil.NewSingleHostReverseProxy(vhost)
		rp.Transport = transport
		pattern := name + "/"
		mux.HandleFunc(pattern, rp.ServeHTTP)
		log.Printf("vhost proxy %s:%d", name, svc.TargetPort())
	}

	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", o.address, o.port))
	if err != nil {
		return err
	}
	log.Printf("Starting to serve on %s\n", l.Addr().String())
	server := &http.Server{Handler: mux}
	go server.Serve(l)
	<-ctx.Done()
	server.Close()
	return nil
}

func main() {
	o := NewVhostProxyOptions()
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
