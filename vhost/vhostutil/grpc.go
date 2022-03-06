package vhostutil

import (
	"github.com/josudoey/kube"
	"github.com/josudoey/kube/vhost"
	"google.golang.org/grpc"
	"k8s.io/client-go/rest"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func GRPCPortForwardFor(resolver *vhost.PortForwardResolver, restClient rest.Interface, config *rest.Config, namespace string) grpc.DialOption {
	return grpc.WithContextDialer(
		PortForwardDialer(resolver, restClient, config, namespace),
	)
}

func GRPCPortForward(f cmdutil.Factory, opts ...kube.KubeOption) (grpc.DialOption, error) {
	o := kube.NewKubeOptions(opts)
	config, err := f.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	client, err := kube.GetClient(f)
	if err != nil {
		return nil, err
	}

	resolver, err := NewPortForwardResolverAndPull(f, opts...)
	if err != nil {
		return nil, err
	}

	return GRPCPortForwardFor(resolver, client.RESTClient(), config, o.Namespace), nil
}
