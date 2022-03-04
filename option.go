package kube

type KubeOptions struct {
	Namespace       string
	LabelSelector   string
	ResourceVersion string
}

type KubeOption func(*KubeOptions)

func WithNamespace(namespace string) KubeOption {
	return func(o *KubeOptions) {
		o.Namespace = namespace
	}
}

func WithLabelSelector(selector string) KubeOption {
	return func(o *KubeOptions) {
		o.LabelSelector = selector
	}
}

func WithResourceVersion(resourceVersion string) KubeOption {
	return func(o *KubeOptions) {
		o.ResourceVersion = resourceVersion
	}
}
