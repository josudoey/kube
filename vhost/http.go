package vhost

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
)

func (resolver *PortForwardResolver) NewRoundTripper(client rest.Interface, config *rest.Config, namespace string) http.RoundTripper {
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				host = addr
			}
			backend := resolver.ResolveBackend(host)
			if backend == nil {
				err := fmt.Errorf("%s svc not found", addr)
				runtime.HandleError(err)
				return nil, err
			}

			conn, err := backend.DialPortForwardOnce(client, config, namespace)
			if err != nil {
				resolver.DeleteByName(backend.GetName())
				return nil, err
			}
			conn.OnCreateStream = backend.OnCreateStream
			conn.OnCloseStream = backend.OnCloseStream

			local, remote := net.Pipe()
			go func() {
				defer local.Close()
				err := conn.Forward(remote, uint16(backend.GetTargetPort()), nil)
				if err != nil {
					resolver.DeleteByName(backend.GetName())
				}
			}()

			return local, nil
		},
	}
}
