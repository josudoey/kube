## kube-pod-watcher usage

```
$ go install github.com/josudoey/kube/cmd/kube-pod-watcher@v0.0.5
$ kube-pod-watcher -h
$ kube-pod-watcher
```


## kube-vhost usage

```
$ go install github.com/josudoey/kube/cmd/kube-vhost@v0.0.5
$ kube-vhost -h
$ kube-vhost show
$ kube-vhost server --port 8010
```


### http client portforwad example

```golang
package main

import (
	"net/http"

	"github.com/go-resty/resty/v2"
	"github.com/josudoey/kube/kubeutil"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)


func main() {
	vhostPortForward, _ := kubeutil.HTTPPortForward(kubeutil.DefaultFactory())
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
	"github.com/josudoey/kube/kubeutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)


func main() {
	vhostPortForward, _ := vhostutil.GRPCPortForward(kubeutil.DefaultFactory())
	addr := "<service name>:<port>"
	conn, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		vhostPortForward,
	)
	// ...
}
```
