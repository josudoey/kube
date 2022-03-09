package kubeutil

import (
	"context"
	"net"
	"strconv"

	"github.com/josudoey/kube"
	"github.com/josudoey/kube/vhost"
	v1 "k8s.io/api/core/v1"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func PortForwardDialer(resolver *vhost.PortForwardResolver, restClient rest.Interface, config *rest.Config, namespace string) func(ctx context.Context, addr string) (net.Conn, error) {
	return func(ctx context.Context, addr string) (net.Conn, error) {
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
	}
}

func PullServices(ctx context.Context, resolver *vhost.PortForwardResolver, client coreclient.ServicesGetter, opts ...kube.KubeOption) (*v1.ServiceList, error) {
	serviceList, err := kube.GetServiceList(ctx, client,
		opts...,
	)
	if err != nil {
		return nil, err
	}

	for _, svc := range serviceList.Items {
		resolver.AddService(svc)
	}
	return serviceList, nil
}

func PullPods(ctx context.Context, resolver *vhost.PortForwardResolver, client coreclient.PodsGetter, opts ...kube.KubeOption) (*v1.PodList, error) {
	podList, err := kube.GetPodList(ctx, client,
		opts...,
	)
	if err != nil {
		return nil, err
	}

	for _, pod := range podList.Items {
		resolver.AddPod(pod)
	}
	return podList, nil
}

func Pull(ctx context.Context, resolver *vhost.PortForwardResolver, client coreclient.CoreV1Interface, opts ...kube.KubeOption) error {
	_, err := PullServices(ctx, resolver, client, opts...)
	if err != nil {
		return err
	}

	_, err = PullPods(ctx, resolver, client, opts...)
	if err != nil {
		return err
	}
	return nil
}

func NewPortForwardResolverAndPull(f cmdutil.Factory, opts ...kube.KubeOption) (*vhost.PortForwardResolver, error) {
	ctx := context.Background()
	resolver := vhost.NewPortForwardResolver()
	client, err := kube.GetClient(f)
	if err != nil {
		return nil, err
	}

	err = Pull(ctx, resolver, client, opts...)
	if err != nil {
		return nil, err
	}

	return resolver, nil
}
