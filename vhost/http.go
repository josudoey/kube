package vhost

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
)

func (resolver *HttpPortForwardResolver) GetHttpTransport(client *rest.RESTClient, config *rest.Config, namespace string) *http.Transport {
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}

			director := resolver.LookupServiceDirector(host)
			if director == nil {
				// TODO http 503
				err = fmt.Errorf("%s svc not found", host)
				runtime.HandleError(err)
				return nil, err
			}

			backend := director.LookupPodBackend()
			if backend == nil {
				err := fmt.Errorf("%s pod not found", director.name)
				return nil, err
			}

			conn, err := backend.DialPortForwardOnce(client, config, namespace)
			if err != nil {
				director.Evict(backend.Name())
				return nil, err
			}

			local, remote := net.Pipe()
			go func() {
				defer local.Close()
				err := conn.Forward(remote, director.TargetPort(), nil, nil)
				if err != nil {
					director.Evict(backend.Name())
				}
			}()

			return local, nil
		},
	}
}
