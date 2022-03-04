package kube

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubectl/pkg/util/podutils"
)

var IsPodReady = podutils.IsPodReady

func GetPodList(ctx context.Context, client coreclient.PodsGetter, opts ...KubeOption) (*corev1.PodList, error) {
	o := &KubeOptions{}
	for _, opt := range opts {
		opt(o)
	}
	options := metav1.ListOptions{LabelSelector: o.LabelSelector}
	return client.Pods(o.Namespace).List(ctx, options)
}

// GetFirstPod returns a pod matching the namespace and label selector
// and the number of all pods that match the label selector.
func GetPods(ctx context.Context, client coreclient.PodsGetter, opts ...KubeOption) ([]*corev1.Pod, error) {
	podList, err := GetPodList(ctx, client, opts...)
	if err != nil {
		return nil, err
	}
	pods := []*corev1.Pod{}
	for i := range podList.Items {
		pod := podList.Items[i]
		pods = append(pods, &pod)
	}
	return pods, nil
}
