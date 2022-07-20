## kube-pod-watcher usage

```
$ go install github.com/josudoey/kube/cmd/kube-pod-watcher@v0.0.6
$ kube-pod-watcher -h
$ kube-pod-watcher
```


## kube-vhost usage

```
$ go install github.com/josudoey/kube/cmd/kube-vhost@v0.0.6
$ kube-vhost -h
$ kube-vhost show
$ kube-vhost server --port 8010
```


## kube-info usage

```
$ go install github.com/josudoey/kube/cmd/kube-info@v0.0.6
$ kube-info -h
$ kube-info pod-image
```


### http client portforward example

```golang
package main

import (
	"net/http"

	"github.com/go-resty/resty/v2"
	"github.com/josudoey/kube/kubeutil"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)


func main() {
	kubePortForward, _ := kubeutil.HTTPPortForward(kubeutil.DefaultFactory())
	client := resty.NewWithClient(&http.Client{
		Transport: kubePortForward,
	})
	// ...
}
```



### grpc client portforward example

```golang
package main

import (
	"github.com/josudoey/kube/kubeutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)


func main() {
	kubePortForward, _ := kubeutil.GRPCPortForward(kubeutil.DefaultFactory())
	addr := "<service name>:<port>"
	conn, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		kubePortForward,
	)
	// ...
}
```
