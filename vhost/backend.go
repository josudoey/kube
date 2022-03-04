package vhost

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

type PodMap struct {
	m sync.Map
}

func (p *PodMap) Get(key string) (*corev1.Pod, bool) {
	v, ok := p.m.Load(key)
	if !ok {
		return nil, false
	}
	actual, _ := v.(*corev1.Pod)
	return actual, true
}

func (p *PodMap) GetOrSet(key string, value *corev1.Pod) (actual *corev1.Pod, loaded bool) {
	v, loaded := p.m.LoadOrStore(key, value)
	actual, _ = v.(*corev1.Pod)
	return actual, loaded
}

func (p *PodMap) Delete(key string) {
	p.m.Delete(key)
}

type PodBackend struct {
	matchedPod *MatchedPod
	dialOnce   sync.Once
	connection *PortForwardConnection
	err        error

	OnCreatePortForward func()
	OnClosePortForward  func()
	OnCreateStream      func(id int)
	OnCloseStream       func(id int)
}

func (backend *PodBackend) GetName() string {
	return backend.matchedPod.GetName()
}

func (backend *PodBackend) GetTargetHostPort() string {
	return backend.matchedPod.GetTargetHostPort()
}

func (backend *PodBackend) GetTargetPort() int32 {
	return backend.matchedPod.GetTargetPort()
}

func (backend *PodBackend) Close() error {
	if backend.connection == nil {
		return nil
	}
	return backend.connection.Close()
}

func (backend *PodBackend) DialPortForwardOnce(client *rest.RESTClient, config *rest.Config, namespace string) (*PortForwardConnection, error) {
	backend.dialOnce.Do(func() {
		connection, err := DialPortForwardConnection(client, config, namespace, backend.GetName())
		if err != nil {
			backend.err = err
			return
		}
		backend.connection = connection
		if backend.OnCreatePortForward != nil {
			go backend.OnCreatePortForward()
		}
		go func() {
			<-backend.connection.CloseChan()
			if backend.OnClosePortForward == nil {
				return
			}
			go backend.OnClosePortForward()
		}()
	})

	if backend.err != nil {
		return nil, backend.err
	}
	return backend.connection, nil
}

func NewPodBackend(matchedPod *MatchedPod) *PodBackend {
	return &PodBackend{
		matchedPod: matchedPod,
	}
}

type PodBackendSet struct {
	m sync.Map
}

func (p *PodBackendSet) GetOne() *PodBackend {
	var pod *PodBackend
	p.Range(func(value *PodBackend) bool {
		pod = value
		return false
	})
	return pod
}

func (p *PodBackendSet) Range(f func(value *PodBackend) bool) {
	p.m.Range(func(k, v interface{}) bool {
		value, _ := v.(*PodBackend)
		return f(value)
	})
}

func (p *PodBackendSet) Add(value *PodBackend) {
	p.m.Store(value, value)
}

func (p *PodBackendSet) Delete(value *PodBackend) {
	p.m.Delete(value)
}

func (p *PodBackendSet) DeleteByName(podName string) {
	p.Range(func(value *PodBackend) bool {
		if value.GetName() != podName {
			return true
		}
		p.Delete(value)
		return true
	})
}

type ServiceBackend struct {
	m sync.Map
}

func (p *ServiceBackend) Add(key *ServicePortEntry, value *PodBackend) {
	initValue := &PodBackendSet{}
	v, _ := p.m.LoadOrStore(key, initValue)
	set := v.(*PodBackendSet)
	set.Add(value)
}

func (p *ServiceBackend) GetOne(key *ServicePortEntry) *PodBackend {
	set, ok := p.Get(key)
	if !ok {
		return nil
	}
	return set.GetOne()
}

func (p *ServiceBackend) Get(key *ServicePortEntry) (*PodBackendSet, bool) {
	v, ok := p.m.Load(key)
	if !ok {
		return nil, false
	}
	actual, _ := v.(*PodBackendSet)
	return actual, true
}

func (p *ServiceBackend) Range(f func(key *ServicePortEntry, value *PodBackendSet) bool) {
	p.m.Range(func(k, v interface{}) bool {
		key, _ := k.(*ServicePortEntry)
		value, _ := v.(*PodBackendSet)
		return f(key, value)
	})
}

func (p *ServiceBackend) Delete(pod *PodBackend) {
	p.Range(func(key *ServicePortEntry, value *PodBackendSet) bool {
		value.Delete(pod)
		return false
	})
}
