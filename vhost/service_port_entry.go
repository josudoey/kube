package vhost

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type ServicePortEntry struct {
	Service     corev1.Service
	ServicePort corev1.ServicePort
	Selector    labels.Selector
}

func (s *ServicePortEntry) Match(pod corev1.Pod) bool {
	return s.Selector.Matches(labels.Set(pod.Labels))
}

func (s *ServicePortEntry) SourceHostPort() string {
	return s.Service.GetName() + ":" + strconv.Itoa(int(s.ServicePort.Port))
}

func (s *ServicePortEntry) SourceHostName() string {
	return s.Service.GetName() + "-" + strconv.Itoa(int(s.ServicePort.Port))
}
