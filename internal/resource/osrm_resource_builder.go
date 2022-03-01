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
	profilesToBuild := []OSRMProfile{}
	builders := []ResourceBuilder{}

	if builder.Instance.Spec.Profiles.Driving != nil {
		profilesToBuild = append(profilesToBuild, DrivingProfile)
	}

	if builder.Instance.Spec.Profiles.Cycling != nil {
		profilesToBuild = append(profilesToBuild, CyclingProfile)
	}

	if builder.Instance.Spec.Profiles.Foot != nil {
		profilesToBuild = append(profilesToBuild, FootProfile)
	}

	for _, profile := range profilesToBuild {
		builders = append(builders, []ResourceBuilder{
			builder.Service(profile),
			//builder.PersistentVolumeClaim(profile),
			//builder.Deployment(profile),
			//builder.HorizontalPodAutoscaler(profile),
		}...)
	}

	if len(builders) > 0 {
		builders = append(builders, builder.Ingress(profilesToBuild))
	}

	return builders
}
