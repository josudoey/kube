package vhostutil

import (
	"context"
	"net"
	"net/http"

	"github.com/josudoey/kube"
	"github.com/josudoey/kube/vhost"
	"k8s.io/client-go/rest"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func HTTPPortFordwardFor(resolver *vhost.PortForwardResolver, restClient rest.Interface, config *rest.Config, namespace string) http.RoundTripper {
	dialPortForwad := PortForwadDialer(resolver, restClient, config, namespace)
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialPortForwad(ctx, addr)
		},
	}
}

func HTTPPortForward(f cmdutil.Factory, opts ...kube.KubeOption) (http.RoundTripper, error) {
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

	return HTTPPortFordwardFor(resolver, client.RESTClient(), config, o.Namespace), nil
}
