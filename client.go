package kube

import (
	"k8s.io/cli-runtime/pkg/genericclioptions"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

// see https://github.com/kubernetes/kubectl/blob/652881798563c00c1895ded6ced819030bfaa4d7/pkg/polymorphichelpers/attachablepodforobject.go#L32
func GetClient(restClientGetter genericclioptions.RESTClientGetter) (corev1client.CoreV1Interface, error) {
	clientConfig, err := restClientGetter.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	return corev1client.NewForConfig(clientConfig)
}
