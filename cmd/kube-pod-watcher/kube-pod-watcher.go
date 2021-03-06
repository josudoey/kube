package main

import (
	"context"
	"log"

	"github.com/josudoey/kube"
	"github.com/josudoey/kube/kubeutil"
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
	podList, err := kube.GetPodList(ctx, client,
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

	eventChan := watcher.ResultChan()
	for e := range eventChan {
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
	return nil
}

func main() {
	o := NewPodWatcherOptions()
	f := kubeutil.DefaultFactory()

	cmd := &cobra.Command{
		Use: "kube-pod-watcher",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Run(f, cmd, args))
		},
	}

	cmd.Flags().StringVarP(&o.LabelSelector, "selector", "l", o.LabelSelector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	cmd.Execute()
}
