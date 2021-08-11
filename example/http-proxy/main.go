package main

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/josudoey/kube"
	"github.com/josudoey/kube/proxy"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func Run() error {
	selector := ""
	f := kube.GetFactory()
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
	resolver := proxy.NewPortForwardResolver()
	for _, svc := range serviceList.Items {
		for _, svcPort := range kube.GetServicePorts(&svc) {
			if svcPort.Name != "http" && svcPort.Port != 80 {
				continue
			}

			targetPort := uint16(svcPort.TargetPort.IntVal)
			log.Printf("PortForward %s:%d", svc.Name, targetPort)
			resolver.AddServiceMap(&svc, targetPort)
			break
		}
	}

	resolver.OnAddPodBackend = func(backend *proxy.PodBackend) {
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
				// Watch Pod Use
				// log.Printf("event: %v %v %v %v Ready: %v", e.Type, pod.ObjectMeta.Name, pod.Name, pod.Status.Phase, kube.IsPodReady(pod))
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
	for _, svc := range resolver.GetServiceNames() {
		vhost, err := url.Parse("http://" + svc)
		if err != nil {
			continue
		}
		rp := httputil.NewSingleHostReverseProxy(vhost)
		rp.Transport = transport
		pattern := svc + "/"
		mux.HandleFunc(pattern, rp.ServeHTTP)
	}
	listenAddr := ":8010"
	log.Printf("Listen On %s", listenAddr)
	server := &http.Server{Addr: listenAddr, Handler: mux}
	go server.ListenAndServe()
	<-ctx.Done()
	server.Close()
	return nil
}

func main() {
	err := Run()
	if err != nil {
		log.Fatal(err)
	}
}
