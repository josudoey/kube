package main

import (
	"context"
	"log"

	"github.com/josudoey/kube"
	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type PodWatcherOptions struct {
	LabelSelector string
}

func NewPodWatcherOptions() *PodWatcherOptions {
	return &PodWatcherOptions{}
}

func (o *PodWatcherOptions) Run(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
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
	podList, err := kube.GetPodList(ctx, client, namespace, selector)
	watcher, err := kube.GetPodWatcher(ctx, client, namespace, selector, podList.ResourceVersion)
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
			ready := ""
			if kube.IsPodReady(pod) {
				ready = "(Ready)"
			}
			log.Printf("Event: %s %s%v %s", e.Type, pod.Status.Phase, ready, pod.Name)
		}
	}
}

func main() {
	o := NewPodWatcherOptions()
	f := kube.GetDefaultFactory()

	cmd := &cobra.Command{
		Use: "kube-pod-watcher",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Run(f, cmd, args))
		},
	}

	cmd.Flags().StringVarP(&o.LabelSelector, "selector", "l", o.LabelSelector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	cmd.Execute()
}
