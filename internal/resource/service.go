package resource

import "sigs.k8s.io/controller-runtime/pkg/client"

type ServiceBuilder struct {
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) Service() *ServiceBuilder {
	return &ServiceBuilder{builder}
}

func (builder *ServiceBuilder) Build() (client.Object, error) {
	return nil, nil
}

func (builder *ServiceBuilder) Update(object client.Object) error {
	return nil
}
