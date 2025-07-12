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
}

type OSRMResourceBuilder struct {
	Instance      *osrmv1alpha1.OSRMCluster
	Scheme        *runtime.Scheme
	MapGeneration string
}

type ProfileScopedBuilder struct {
	profile *osrmv1alpha1.ProfileSpec
}

type ClusterScopedBuilder struct {
	profiles []*osrmv1alpha1.ProfileSpec
}

// ResourceBuildersForPhase returns builders based on the current phase
func (builder *OSRMResourceBuilder) ResourceBuildersForPhase(phase osrmv1alpha1.Phase) []ResourceBuilder {
	switch phase {
	case osrmv1alpha1.PhaseBuildingMap:
		return builder.MapBuildingPhaseBuilders()
	case osrmv1alpha1.PhaseDeployingWorkers:
		return builder.DeployingWorkersPhaseBuilders()
	case osrmv1alpha1.PhaseWorkersDeployed:
		return builder.WorkersDeployedPhaseBuilders()
	default:
		// For backwards compatibility during transition
		return builder.ResourceBuilders()
	}
}

// MapBuildingPhaseBuilders returns builders for the map building phase
func (builder *OSRMResourceBuilder) MapBuildingPhaseBuilders() []ResourceBuilder {
	builders := []ResourceBuilder{}

	for _, profile := range builder.Instance.Spec.Profiles {
		builders = append(builders, []ResourceBuilder{
			builder.PersistentVolumeClaim(profile),
			builder.Job(profile),
		}...)
	}

	return builders
}

// DeployingWorkersPhaseBuilders returns builders for the deploying workers phase
func (builder *OSRMResourceBuilder) DeployingWorkersPhaseBuilders() []ResourceBuilder {
	builders := []ResourceBuilder{}

	for _, profile := range builder.Instance.Spec.Profiles {
		builders = append(builders, []ResourceBuilder{
			builder.Deployment(profile),
			builder.Service(profile),
			builder.PodDisruptionBudget(profile),
			builder.HorizontalPodAutoscaler(profile),
		}...)
	}

	return builders
}

// WorkersDeployedPhaseBuilders returns builders for the workers deployed phase
func (builder *OSRMResourceBuilder) WorkersDeployedPhaseBuilders() []ResourceBuilder {
	builders := []ResourceBuilder{}

	// Add CronJobs for speed updates if configured
	for _, profile := range builder.Instance.Spec.Profiles {
		if profile.SpeedUpdates != nil {
			builders = append(builders, builder.CronJob(profile))
		}
	}

	// Add gateway resources
	if len(builder.Instance.Spec.Profiles) > 0 {
		builders = append(builders, []ResourceBuilder{
			builder.ConfigMap(builder.Instance.Spec.Profiles),
			builder.GatewayService(builder.Instance.Spec.Profiles),
			builder.GatewayDeployment(builder.Instance.Spec.Profiles),
		}...)
	}

	return builders
}

// ResourceBuilders returns all builders (for backwards compatibility)
func (builder *OSRMResourceBuilder) ResourceBuilders() []ResourceBuilder {
	builders := []ResourceBuilder{}

	for _, profile := range builder.Instance.Spec.Profiles {
		builders = append(builders, []ResourceBuilder{
			builder.PersistentVolumeClaim(profile),
			builder.Job(profile),
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

func (builder *OSRMResourceBuilder) MapUpdateResourceBuilders() []ResourceBuilder {
	builders := []ResourceBuilder{}

	for _, profile := range builder.Instance.Spec.Profiles {
		builders = append(builders, []ResourceBuilder{
			builder.PersistentVolumeClaim(profile),
			builder.Job(profile),
		}...)
	}

	return builders
}
