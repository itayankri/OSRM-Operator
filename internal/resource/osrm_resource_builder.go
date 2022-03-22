package resource

import (
	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type OSRMService string

type ResourceBuilder interface {
	Build() (client.Object, error)
	Update(client.Object) error
}

type OSRMResourceBuilder struct {
	Instance *osrmv1alpha1.OSRMCluster
	Scheme   *runtime.Scheme
}

type ProfileScopedBuilder struct {
	profile string
}

type ClusterScopedBuilder struct {
	profiles []string
}

func (builder *OSRMResourceBuilder) ResourceBuilders() []ResourceBuilder {
	builders := []ResourceBuilder{}
	profilesEndpoints := []string{}

	for _, profile := range builder.Instance.Spec.Profiles {
		profilesEndpoints = append(profilesEndpoints, profile.EndpointName)
		builders = append(builders, []ResourceBuilder{
			builder.Service(profile.Name),
			builder.Deployment(profile.Name),
			builder.Job(profile.Name),
			builder.HorizontalPodAutoscaler(profile.Name),
			builder.PersistentVolumeClaim(profile.Name),
		}...)
	}

	if len(builders) > 0 && builder.Instance.Spec.Ingress != nil {
		builders = append(builders, builder.Ingress(profilesEndpoints))
	}

	return builders
}
