package kube

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// see https://github.com/kubernetes/kubectl/blob/master/pkg/polymorphichelpers/helpers.go#L84

// GetPod
func GetPod(object runtime.Object) *corev1.Pod {
	switch t := object.(type) {
	case *corev1.Pod:
		return t
	}
	return nil
}

// GetService
func GetService(object runtime.Object) *corev1.Service {
	switch t := object.(type) {
	case *corev1.Service:
		return t
	}
	return nil
}
