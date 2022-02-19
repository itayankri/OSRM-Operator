package resource

import "sigs.k8s.io/controller-runtime/pkg/client"

type HorizontalPodAutoscalerBuilder struct {
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) HorizontalPodAutoscaler() *HorizontalPodAutoscalerBuilder {
	return &HorizontalPodAutoscalerBuilder{builder}
}

func (builder *HorizontalPodAutoscalerBuilder) Build() (client.Object, error) {
	return nil, nil
}

func (builder *HorizontalPodAutoscalerBuilder) Update(object client.Object) error {
	return nil
}
