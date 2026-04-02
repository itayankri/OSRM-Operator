package resource

import (
	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
)

// OSRMInstanceResourceBuilder builds resources for an OSRMInstance (single-profile,
// no NGINX gateway). It reuses all non-gateway builders by delegating to an inner
// OSRMResourceBuilder whose Instance is set to the OSRMInstance itself.
type OSRMInstanceResourceBuilder struct {
	Instance            *osrmv1alpha1.OSRMInstance
	Scheme              *runtime.Scheme
	MapGeneration       string
	FutureMapGeneration string
	inner               *OSRMResourceBuilder
}

func NewOSRMInstanceResourceBuilder(
	instance *osrmv1alpha1.OSRMInstance,
	scheme *runtime.Scheme,
	mapGeneration string,
	futureMapGeneration string,
) *OSRMInstanceResourceBuilder {
	currentMapGeneration := mapGeneration
	if currentMapGeneration == "" {
		currentMapGeneration = "1"
	}
	if futureMapGeneration == "" {
		futureMapGeneration = currentMapGeneration
	}

	inner := &OSRMResourceBuilder{
		Instance:            instance, // *OSRMInstance satisfies OSRMResourceInstance
		Scheme:              scheme,
		MapGeneration:       currentMapGeneration,
		FutureMapGeneration: futureMapGeneration,
	}

	return &OSRMInstanceResourceBuilder{
		Instance:            instance,
		Scheme:              scheme,
		MapGeneration:       currentMapGeneration,
		FutureMapGeneration: futureMapGeneration,
		inner:               inner,
	}
}

// toProfileSpec synthesizes a ProfileSpec from OSRMInstanceSpec.
// profile.Name is empty so that ChildResourceName("", suffix) produces just instance.Name.
// profile.OSRMProfile carries the actual OSRM profile name for map building and the readiness probe.
func (b *OSRMInstanceResourceBuilder) toProfileSpec() *osrmv1alpha1.ProfileSpec {
	return &osrmv1alpha1.ProfileSpec{
		Name:              "",
		OSRMProfile:       &b.Instance.Spec.OSRMProfile,
		Replicas:          b.Instance.Spec.Replicas,
		MinReplicas:       b.Instance.Spec.MinReplicas,
		MaxReplicas:       b.Instance.Spec.MaxReplicas,
		Resources:         b.Instance.Spec.Resources,
		SpeedUpdates:      b.Instance.Spec.SpeedUpdates,
		OSRMRoutedOptions: b.Instance.Spec.OSRMRoutedOptions,
	}
}

func (b *OSRMInstanceResourceBuilder) ResourceBuildersForPhase(phase osrmv1alpha1.Phase) []ResourceBuilder {
	switch phase {
	case osrmv1alpha1.PhaseBuildingMap:
		return b.MapBuildingPhaseBuilders()
	case osrmv1alpha1.PhaseDeployingWorkers:
		return b.DeployingWorkersPhaseBuilders()
	case osrmv1alpha1.PhaseWorkersDeployed, osrmv1alpha1.PhaseWorkersRedeployed:
		return b.WorkersDeployedPhaseBuilders()
	case osrmv1alpha1.PhaseUpdatingMap:
		return b.MapUpdatingResourceBuilders()
	case osrmv1alpha1.PhaseRedepoloyingWorkers:
		return b.RedeployingWorkersPhaseBuilders()
	default:
		return nil
	}
}

func (b *OSRMInstanceResourceBuilder) MapBuildingPhaseBuilders() []ResourceBuilder {
	profile := b.toProfileSpec()
	return []ResourceBuilder{
		b.inner.PersistentVolumeClaim(profile, b.MapGeneration),
		b.inner.Job(profile, b.MapGeneration),
	}
}

func (b *OSRMInstanceResourceBuilder) MapUpdatingResourceBuilders() []ResourceBuilder {
	profile := b.toProfileSpec()
	nextMapGeneration := getNextMapGeneration(b.MapGeneration)
	return []ResourceBuilder{
		b.inner.PersistentVolumeClaim(profile, nextMapGeneration),
		b.inner.Job(profile, nextMapGeneration),
	}
}

func (b *OSRMInstanceResourceBuilder) RedeployingWorkersPhaseBuilders() []ResourceBuilder {
	profile := b.toProfileSpec()
	return []ResourceBuilder{
		b.inner.Deployment(profile, b.FutureMapGeneration),
	}
}

func (b *OSRMInstanceResourceBuilder) DeployingWorkersPhaseBuilders() []ResourceBuilder {
	profile := b.toProfileSpec()
	return []ResourceBuilder{
		b.inner.Deployment(profile, b.MapGeneration),
		b.inner.Service(profile),
		b.inner.PodDisruptionBudget(profile),
		b.inner.HorizontalPodAutoscaler(profile),
	}
}

// WorkersDeployedPhaseBuilders returns builders for the steady-state phase.
// Unlike OSRMResourceBuilder, no gateway resources are included.
func (b *OSRMInstanceResourceBuilder) WorkersDeployedPhaseBuilders() []ResourceBuilder {
	profile := b.toProfileSpec()
	builders := []ResourceBuilder{
		b.inner.Service(profile),
		b.inner.HorizontalPodAutoscaler(profile),
		b.inner.PodDisruptionBudget(profile),
		b.inner.Deployment(profile, b.MapGeneration),
	}
	if b.Instance.Spec.SpeedUpdates != nil {
		builders = append(builders, b.inner.CronJob(profile, b.MapGeneration))
	}
	return builders
}
