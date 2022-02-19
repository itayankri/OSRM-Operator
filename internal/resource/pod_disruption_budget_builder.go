package resource

import "sigs.k8s.io/controller-runtime/pkg/client"

type PodDisruptionBudgetBuilder struct {
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) PodDisruptionBudget() *PodDisruptionBudgetBuilder {
	return &PodDisruptionBudgetBuilder{builder}
}

func (builder *PodDisruptionBudgetBuilder) Build() (client.Object, error) {
	return nil, nil
}

func (builder *PodDisruptionBudgetBuilder) Update(object client.Object) error {
	return nil
}
