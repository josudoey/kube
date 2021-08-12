package kube

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	watch "k8s.io/apimachinery/pkg/watch"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
)

// GetFirstPod returns a pod matching the namespace and label selector
// and the number of all pods that match the label selector.
// see https://github.com/kubernetes/apiserver/blob/92392ef22153d75b3645b0ae339f89c12767fb52/pkg/endpoints/handlers/watch.go
func GetPodWatcher(ctx context.Context, client coreclient.PodsGetter, namespace string, selector string, podList *v1.PodList) (watch.Interface, error) {
	options := metav1.ListOptions{LabelSelector: selector}
	options.ResourceVersion = podList.ResourceVersion
	return client.Pods(namespace).Watch(ctx, options)
}
