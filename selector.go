package kube

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/polymorphichelpers"
)

var SelectorsForObject = polymorphichelpers.SelectorsForObject

func GetPodSelector(f cmdutil.Factory, namespace string, resourceName string) (labels.Selector, error) {
	builder := f.NewBuilder().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		ContinueOnError().
		NamespaceParam(namespace).DefaultNamespace()

	builder.ResourceNames("pods", resourceName)

	obj, err := builder.Do().Object()
	if err != nil {
		return nil, err
	}

	_, selector, err := SelectorsForObject(obj)
	if err != nil {
		return nil, err
	}

	return selector, nil
}
