## kube-pod-watcher usage

```
$ go install github.com/josudoey/kube/cmd/kube-pod-watcher@v0.0.3
$ kube-pod-watcher -h
$ kube-pod-watcher
```


## kube-vhost usage

```
$ go install github.com/josudoey/kube/cmd/kube-vhost@v0.0.3
$ kube-vhost -h
$ kube-vhost show
$ kube-vhost server --port 8010
```


### http client portforwad example

```golang
package main

import (
	"net/http"

	"github.com/josudoey/kube"
	"github.com/josudoey/kube/vhost/vhostutil"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)


func main() {
	vhostPortForward, _ := vhostutil.HTTPPortForward(kube.DefaultFactory())
	client := resty.NewWithClient(&http.Client{
		Transport: vhostPortForward,
	})
	// ...
}
```



### grpc client portforwad example

```golang
package main

import (
	"github.com/josudoey/kube"
	"github.com/josudoey/kube/vhost/vhostutil"
	"google.golang.org/grpc"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)


func main() {
	vhostPortForward, err := vhostutil.GRPCPortForward(kube.DefaultFactory())
	if err != nil {
		return nil, err
	}
	addr := "<svc-name>:<port>"
	conn, err := grpc.Dial(addr,
		grpc.WithInsecure(),
		vhostPortForward,
	)
	// ...
}
```