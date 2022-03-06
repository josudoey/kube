package kube

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	watch "k8s.io/apimachinery/pkg/watch"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
)

// GetFirstPod returns a pod matching the namespace and label selector
// and the number of all pods that match the label selector.
// see https://github.com/kubernetes/apiserver/blob/92392ef22153d75b3645b0ae339f89c12767fb52/pkg/endpoints/handlers/watch.go
func GetPodWatcher(ctx context.Context, client coreclient.PodsGetter, opts ...KubeOption) (watch.Interface, error) {
	o := NewKubeOptions(opts)
	options := metav1.ListOptions{
		LabelSelector:   o.LabelSelector,
		ResourceVersion: o.ResourceVersion,
	}
	return client.Pods(o.Namespace).Watch(ctx, options)
}
