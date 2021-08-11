package kube

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubectl/pkg/util/podutils"
)

var IsPodReady = podutils.IsPodReady

func GetPodList(ctx context.Context, client coreclient.PodsGetter, namespace string, selector string) (*corev1.PodList, error) {
	options := metav1.ListOptions{LabelSelector: selector}
	return client.Pods(namespace).List(ctx, options)
}

// GetFirstPod returns a pod matching the namespace and label selector
// and the number of all pods that match the label selector.
func GetPods(ctx context.Context, client coreclient.PodsGetter, namespace string, selector string) ([]*corev1.Pod, error) {
	podList, err := GetPodList(ctx, client, namespace, selector)
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
