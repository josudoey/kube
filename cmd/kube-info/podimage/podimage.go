package podimage

import (
	"context"
	"fmt"

	"github.com/josudoey/kube"
	"github.com/josudoey/kube/kubeutil"
	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type KubeInfoPodImageOptions struct {
	LabelSelector string
}

func NewKubeInfoPodImageOptions() *KubeInfoPodImageOptions {
	return &KubeInfoPodImageOptions{}
}

func (o *KubeInfoPodImageOptions) Run(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
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
	pods, err := kube.GetPodList(ctx, client,
		kube.WithNamespace(namespace),
		kube.WithLabelSelector(selector),
	)
	if err != nil {
		return err
	}

	for _, pod := range pods.Items {
		for _, container := range pod.Spec.InitContainers {
			fmt.Printf("%v\t%v\n", pod.Name, container.Image)
		}
		for _, container := range pod.Spec.Containers {
			fmt.Printf("%v\t%v\n", pod.Name, container.Image)
		}
	}

	return nil
}

func NewCommand() *cobra.Command {
	o := NewKubeInfoPodImageOptions()
	f := kubeutil.DefaultFactory()

	cmd := &cobra.Command{
		Use: "pod-image",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Run(f, cmd, args))
		},
	}

	cmd.Flags().StringVarP(&o.LabelSelector, "selector", "l", o.LabelSelector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	return cmd
}
