package resource

import (
	osrmv1alpha1 "github.com/itayankri/OSRM-Opeator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ResourceBuilder interface {
	Build() (client.Object, error)
	Update(client.Object) error
}

type OSRMResourceBuilder struct {
	Instance *osrmv1alpha1.OSRMCluster
	Scheme   *runtime.Scheme
}

func (builder *OSRMResourceBuilder) ResourceBuilders() []ResourceBuilder {
	builders := []ResourceBuilder{
		builder.Service(),
		builder.PersistentVolumeClaim(),
		builder.Deployment(),
		builder.HorizontalPodAutoscaler(),
		builder.PodDisruptionBudget(),
		builder.Ingress(),
	}
	return builders
}
