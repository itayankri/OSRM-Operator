package resource

import "sigs.k8s.io/controller-runtime/pkg/client"

type IngressBuilder struct {
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) Ingress() *IngressBuilder {
	return &IngressBuilder{builder}
}

func (builder *IngressBuilder) Build() (client.Object, error) {
	return nil, nil
}

func (builder *IngressBuilder) Update(object client.Object) error {
	return nil
}
