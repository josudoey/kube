package vhostutil

import (
	"context"
	"net"
	"strconv"

	"github.com/josudoey/kube"
	"github.com/josudoey/kube/vhost"
	"google.golang.org/grpc"
	"k8s.io/client-go/rest"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func GRPCPortForwardFor(resolver *vhost.PortForwardResolver, restClient rest.Interface, config *rest.Config, namespace string) grpc.DialOption {
	return grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(resolver.ResolveAddr(addr))
		if err != nil {
			return nil, err
		}

		conn, err := vhost.DialPortForwardConnection(restClient, config, namespace, host)
		if err != nil {
			return nil, err
		}

		local, remote := net.Pipe()
		go func() {
			defer local.Close()
			p, _ := strconv.ParseUint(port, 10, 16)
			conn.Forward(remote, uint16(p), nil)
		}()
		return local, nil
	})
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
