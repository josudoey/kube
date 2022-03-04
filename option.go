package kube

type KubeOptions struct {
	Namespace       string
	LabelSelector   string
	ResourceVersion string
}

type KubeOption func(*KubeOptions)

func GetKubeOptions(opts []KubeOption) *KubeOptions {
	o := &KubeOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

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
