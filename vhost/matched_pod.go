package vhost

import (
	"strconv"

	v1 "k8s.io/api/core/v1"
)

type MatchedPod struct {
	v1.ServicePort
	v1.Pod
}

func (r *MatchedPod) GetName() string {
	return r.Pod.GetName()
}

func (r *MatchedPod) GetTargetPort() int32 {
	if r.ServicePort.TargetPort.IntVal != 0 {
		return r.ServicePort.TargetPort.IntVal
	}
	if port, err := strconv.Atoi(r.ServicePort.TargetPort.StrVal); err != nil {
		return int32(port)
	}
	return r.ServicePort.TargetPort.IntVal
}

func (r *MatchedPod) GetTargetHostPort() string {
	port := r.ServicePort.TargetPort.StrVal
	if port == "" {
		port = strconv.Itoa(int(r.ServicePort.TargetPort.IntVal))
	}
	return r.Pod.GetName() + ":" + port
}
