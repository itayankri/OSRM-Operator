package resource

import (
	osrmv1alpha1 "github.com/itayankri/OSRM-Opeator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DrivingProfile OSRMProfile = "driving"
	CyclingProfile OSRMProfile = "cycling"
	FootProfile    OSRMProfile = "foot"
)

type OSRMProfile string

type ResourceBuilder interface {
	Build() (client.Object, error)
	Update(client.Object) error
}

type OSRMResourceBuilder struct {
	Instance *osrmv1alpha1.OSRMCluster
	Scheme   *runtime.Scheme
}

type BaseBuilder struct {
	profile OSRMProfile
}

func (builder *OSRMResourceBuilder) ResourceBuilders() []ResourceBuilder {
	builders := []ResourceBuilder{
		builder.Service(DrivingProfile),
		builder.PersistentVolumeClaim(DrivingProfile),
		builder.Deployment(DrivingProfile),
		builder.HorizontalPodAutoscaler(DrivingProfile),

		builder.Service(CyclingProfile),
		builder.PersistentVolumeClaim(CyclingProfile),
		builder.Deployment(CyclingProfile),
		builder.HorizontalPodAutoscaler(CyclingProfile),

		builder.Service(FootProfile),
		builder.PersistentVolumeClaim(FootProfile),
		builder.Deployment(FootProfile),
		builder.HorizontalPodAutoscaler(FootProfile),

		builder.Ingress(),
	}
	return builders
}
