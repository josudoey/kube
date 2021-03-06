package kube

type KubeOptions struct {
	Namespace       string
	LabelSelector   string
	ResourceVersion string
}

type KubeOption func(*KubeOptions)

func NewKubeOptions(opts []KubeOption) *KubeOptions {
	o := &KubeOptions{
		Namespace: "default",
	}
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
