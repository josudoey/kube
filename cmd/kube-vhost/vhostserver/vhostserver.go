package vhostserver

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/josudoey/kube"
	"github.com/josudoey/kube/kubeutil"
	"github.com/josudoey/kube/vhost"
	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

const (
	defaultPort    = 8010
	defaultAddress = "127.0.0.1"
)

type KubeVhostServerOptions struct {
	port    int
	address string
	verbose bool

	LabelSelector string
}

func NewKubeVhostServerOptions() *KubeVhostServerOptions {
	return &KubeVhostServerOptions{
		port:    defaultPort,
		address: defaultAddress,
	}
}

func (o *KubeVhostServerOptions) Run(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
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
	resolver := vhost.NewPortForwardResolver()
	resolver.OnAddServiceBackend = func(entry vhost.ServicePortEntry, backend *vhost.PodBackend) {
		sourceHostName := entry.SourceHostName()
		targetHostPort := backend.GetTargetHostPort()
		if o.verbose {
			log.Printf("Add service backend %s -> %s", sourceHostName, targetHostPort)
		}
		backend.OnCreatePortForward = func() {
			log.Printf("Created PortForward %s -> %s", sourceHostName, targetHostPort)
		}
		backend.OnClosePortForward = func() {
			log.Printf("Closed PortForward %s -> %s", sourceHostName, targetHostPort)
		}
		backend.OnCreateStream = func(id int) {
			log.Printf("Created Stream#%d %s", id, targetHostPort)
		}
		backend.OnCloseStream = func(id int) {
			log.Printf("Closed Stream#%d %s", id, targetHostPort)
		}
	}

	_, err = kubeutil.PullServices(ctx, resolver, client,
		kube.WithNamespace(namespace),
		kube.WithLabelSelector(selector),
	)
	if err != nil {
		return err
	}

	podList, err := kubeutil.PullPods(ctx, resolver, client,
		kube.WithNamespace(namespace),
		kube.WithLabelSelector(selector),
	)

	if err != nil {
		return err
	}

	watcher, err := kube.GetPodWatcher(ctx, client,
		kube.WithNamespace(namespace),
		kube.WithLabelSelector(selector),
		kube.WithResourceVersion(podList.ResourceVersion),
	)
	if err != nil {
		return err
	}
	defer watcher.Stop()

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		ch := watcher.ResultChan()
		for e := range ch {
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
			resolver.UpdatePod(pod)
		}
		cancel()
	}()

	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	roundTripper := resolver.NewRoundTripper(client.RESTClient(), config, namespace)
	for _, svc := range resolver.ListServices() {
		name := svc.SourceHostName()
		vhost, err := url.Parse("http://" + name)
		if err != nil {
			continue
		}
		rp := httputil.NewSingleHostReverseProxy(vhost)
		rp.Transport = roundTripper
		pattern := name + "/"
		mux.HandleFunc(pattern, rp.ServeHTTP)
		log.Printf("vhost port-forward %s -> svc/%s", name, svc.SourceHostPort())
	}

	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", o.address, o.port))
	if err != nil {
		return err
	}

	server := &http.Server{
		Handler: resolver.GetGRPCHandler(mux, client.RESTClient(), config, namespace),
	}
	go server.Serve(l)
	<-ctx.Done()
	server.Close()
	return nil
}

func NewCommand() *cobra.Command {
	o := NewKubeVhostServerOptions()
	f := kubeutil.DefaultFactory()

	cmd := &cobra.Command{
		Use: "server [--port=PORT]",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Run(f, cmd, args))
		},
	}

	cmd.Flags().BoolVarP(&o.verbose, "verbose", "v", o.verbose, "Set verbose mode.")
	cmd.Flags().IntVarP(&o.port, "port", "p", o.port, "The port on which to run the proxy. Set to 0 to pick a random port.")
	cmd.Flags().StringVar(&o.address, "address", o.address, "The IP address on which to serve on.")
	cmd.Flags().StringVarP(&o.LabelSelector, "selector", "l", o.LabelSelector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	return cmd
}
