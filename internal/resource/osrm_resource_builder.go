package resource

import (
	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type OSRMService string

type ResourceBuilder interface {
	Build() (client.Object, error)
	Update(client.Object, []runtime.Object) error
	ShouldDeploy(resources []runtime.Object) bool
}

type OSRMResourceBuilder struct {
	Instance *osrmv1alpha1.OSRMCluster
	Scheme   *runtime.Scheme
}

type ProfileScopedBuilder struct {
	profile *osrmv1alpha1.ProfileSpec
}

type MapScopedBuilder struct {
	generation int32
}

type ClusterScopedBuilder struct {
	profiles []*osrmv1alpha1.ProfileSpec
}

func (builder *OSRMResourceBuilder) ResourceBuilders(mapGeneration string) []ResourceBuilder {
	builders := []ResourceBuilder{}
	for _, profile := range builder.Instance.Spec.Profiles {
		builders = append(builders, []ResourceBuilder{
			builder.PersistentVolumeClaim(profile, mapGeneration),
			builder.Job(profile, mapGeneration),
			builder.Deployment(profile),
			builder.Service(profile),
			builder.CronJob(profile),
			builder.PodDisruptionBudget(profile),
			builder.HorizontalPodAutoscaler(profile),
		}...)
	}

	if len(builders) > 0 {
		builders = append(builders, []ResourceBuilder{
			builder.ConfigMap(builder.Instance.Spec.Profiles),
			builder.GatewayService(builder.Instance.Spec.Profiles),
			builder.GatewayDeployment(builder.Instance.Spec.Profiles),
		}...)
	}

	return builders
}
