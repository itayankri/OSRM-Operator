package resource

import "sigs.k8s.io/controller-runtime/pkg/client"

type DeploymentBuilder struct {
	BaseBuilder
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) Deployment(profile OSRMProfile) *DeploymentBuilder {
	return &DeploymentBuilder{
		BaseBuilder{profile},
		builder,
	}
}

func (builder *DeploymentBuilder) Build() (client.Object, error) {
	return nil, nil
}

func (builder *DeploymentBuilder) Update(object client.Object) error {
	return nil
}
