package vhostshow

import (
	"context"
	"fmt"

	"github.com/josudoey/kube"
	"github.com/josudoey/kube/kubeutil"
	"github.com/josudoey/kube/vhost"
	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type KubeVhostShowOptions struct {
	LabelSelector string
}

func NewKubeVhostShowOptions() *KubeVhostShowOptions {
	return &KubeVhostShowOptions{}
}

func (o *KubeVhostShowOptions) Run(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
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

	err = kubeutil.Pull(ctx, resolver, client,
		kube.WithNamespace(namespace),
		kube.WithLabelSelector(selector),
	)

	if err != nil {
		return err
	}

	for _, svc := range resolver.ListServices() {
		name := svc.SourceHostName()
		fmt.Printf("%s -> svc/%s\n", name, svc.SourceHostPort())
	}

	return nil
}

func NewCommand() *cobra.Command {
	o := NewKubeVhostShowOptions()
	f := kubeutil.DefaultFactory()

	cmd := &cobra.Command{
		Use: "show",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Run(f, cmd, args))
		},
	}

	cmd.Flags().StringVarP(&o.LabelSelector, "selector", "l", o.LabelSelector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	return cmd
}
