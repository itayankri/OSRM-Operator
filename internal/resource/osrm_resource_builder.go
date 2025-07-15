package resource

import (
	"strconv"

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
	Instance            *osrmv1alpha1.OSRMCluster
	Scheme              *runtime.Scheme
	MapGeneration       string
	FutureMapGeneration string
}

func NewOSRMResourceBuilder(
	instance *osrmv1alpha1.OSRMCluster,
	scheme *runtime.Scheme,
	mapGeneration string,
	nextMapGeneration string,
) *OSRMResourceBuilder {
	currentMapGeneration := mapGeneration
	futureMapGeneration := nextMapGeneration

	if mapGeneration == "" {
		currentMapGeneration = "1"
	}

	if futureMapGeneration == "" {
		futureMapGeneration = currentMapGeneration
	}

	return &OSRMResourceBuilder{
		Instance:            instance,
		Scheme:              scheme,
		MapGeneration:       currentMapGeneration,
		FutureMapGeneration: futureMapGeneration,
	}
}

type ProfileScopedBuilder struct {
	profile *osrmv1alpha1.ProfileSpec
}

type MapGenerationScopedBuilder struct {
	generation string
}

type ClusterScopedBuilder struct {
	profiles []*osrmv1alpha1.ProfileSpec
}

func (builder *OSRMResourceBuilder) ResourceBuildersForPhase(phase osrmv1alpha1.Phase) []ResourceBuilder {
	switch phase {
	case osrmv1alpha1.PhaseBuildingMap:
		return builder.MapBuildingPhaseBuilders()
	case osrmv1alpha1.PhaseDeployingWorkers:
		return builder.DeployingWorkersPhaseBuilders()
	case osrmv1alpha1.PhaseWorkersDeployed, osrmv1alpha1.PhaseWorkersRedeployed:
		return builder.WorkersDeployedPhaseBuilders()
	case osrmv1alpha1.PhaseUpdatingMap:
		return builder.MapUpdatingResourceBuilders()
	case osrmv1alpha1.PhaseRedepoloyingWorkers:
		return builder.RedeployingWorkersPhaseBuilders()
	default:
		return nil
	}
}

func (builder *OSRMResourceBuilder) MapBuildingPhaseBuilders() []ResourceBuilder {
	builders := []ResourceBuilder{}

	for _, profile := range builder.Instance.Spec.Profiles {
		builders = append(builders, []ResourceBuilder{
			builder.PersistentVolumeClaim(profile, builder.MapGeneration),
			builder.Job(profile, builder.MapGeneration),
		}...)
	}

	return builders
}

func (builder *OSRMResourceBuilder) MapUpdatingResourceBuilders() []ResourceBuilder {
	nextMapGeneration := getNextMapGeneration(builder.MapGeneration)

	builders := []ResourceBuilder{}

	for _, profile := range builder.Instance.Spec.Profiles {
		builders = append(builders, []ResourceBuilder{
			builder.PersistentVolumeClaim(profile, nextMapGeneration),
			builder.Job(profile, nextMapGeneration),
		}...)
	}

	return builders
}

func (builder *OSRMResourceBuilder) RedeployingWorkersPhaseBuilders() []ResourceBuilder {
	builders := []ResourceBuilder{}

	for _, profile := range builder.Instance.Spec.Profiles {
		builders = append(builders, []ResourceBuilder{
			builder.Deployment(profile, builder.FutureMapGeneration),
		}...)
	}

	return builders
}

func (builder *OSRMResourceBuilder) DeployingWorkersPhaseBuilders() []ResourceBuilder {
	builders := []ResourceBuilder{}

	for _, profile := range builder.Instance.Spec.Profiles {
		builders = append(builders, []ResourceBuilder{
			builder.Deployment(profile, builder.MapGeneration),
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

func getNextMapGeneration(mapGeneration string) string {
	mapGenerationInteger, err := strconv.Atoi(mapGeneration)
	if err != nil {
		return "1"
	}

	return strconv.Itoa(mapGenerationInteger + 1)
}
