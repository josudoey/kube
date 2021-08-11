package kube

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
)

func GetServiceList(ctx context.Context, client coreclient.ServicesGetter, namespace string, selector string) (*corev1.ServiceList, error) {
	options := metav1.ListOptions{LabelSelector: selector}
	return client.Services(namespace).List(ctx, options)
}
