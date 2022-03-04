package vhost

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
)

func (resolver *PortForwardResolver) GetHttpTransport(client *rest.RESTClient, config *rest.Config, namespace string) *http.Transport {
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			backend := resolver.ResolveBackend(addr)
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
