package main

import (
	"context"
	"log"

	"github.com/josudoey/kube"
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
	podList, err := kube.GetPodList(ctx, client, namespace, selector)
	watcher, err := kube.GetPodWatcher(ctx, client, namespace, selector, podList)
	if err != nil {
		return err
	}

	ch := watcher.ResultChan()
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				log.Printf("PodWatcher Closed")
				return nil
			}

			pod := kube.GetPod(e.Object)
			if pod == nil {
				continue
			}
			// Watch Pod Use
			log.Printf("event: %v %v %v %v Ready: %v", e.Type, pod.ObjectMeta.Name, pod.Name, pod.Status.Phase, kube.IsPodReady(pod))
		}
	}
}

func main() {
	err := Run()
	if err != nil {
		log.Fatal(err)
	}
}
