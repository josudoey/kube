package kube

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// see https://github.com/kubernetes/kubectl/blob/master/pkg/polymorphichelpers/helpers.go#L84

// GetService
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

// GetServicePorts
func GetServicePorts(object runtime.Object) []corev1.ServicePort {
	svc := GetService(object)
	if svc == nil {
		return nil
	}
	return svc.Spec.Ports
}

// GetServicePort
func GetServicePort(object runtime.Object, name string) *corev1.ServicePort {
	svc := GetService(object)
	if svc == nil {
		return nil
	}
	for _, port := range svc.Spec.Ports {
		if port.Name != name {
			continue
		}
		return &port
	}
	return nil
}
