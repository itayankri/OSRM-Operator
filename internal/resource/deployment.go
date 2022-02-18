package resource

import "sigs.k8s.io/controller-runtime/pkg/client"

type DeploymentBuilder struct {
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) Deployment() *DeploymentBuilder {
	return &DeploymentBuilder{builder}
}

func (builder *DeploymentBuilder) Build() (client.Object, error) {
	return nil, nil
}

func (builder *DeploymentBuilder) Update(object client.Object) error {
	return nil
}
