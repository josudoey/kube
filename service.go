package kube

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
)

func GetServiceList(ctx context.Context, client coreclient.ServicesGetter, opts ...KubeOption) (*corev1.ServiceList, error) {
	o := NewKubeOptions(opts)
	options := metav1.ListOptions{LabelSelector: o.LabelSelector}

	return client.Services(o.Namespace).List(ctx, options)
}
