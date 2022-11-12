package resource

import (
	"fmt"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/metadata"
	"github.com/itayankri/OSRM-Operator/internal/status"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type HorizontalPodAutoscalerBuilder struct {
	ProfileScopedBuilder
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) HorizontalPodAutoscaler(profile *osrmv1alpha1.ProfileSpec) *HorizontalPodAutoscalerBuilder {
	return &HorizontalPodAutoscalerBuilder{
		ProfileScopedBuilder{profile},
		builder,
	}
}

func (builder *HorizontalPodAutoscalerBuilder) Build() (client.Object, error) {
	return &autoscalingv1.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName(builder.profile.Name, HorizontalPodAutoscalerSuffix),
			Namespace: builder.Instance.Namespace,
		},
	}, nil
}

func (builder *HorizontalPodAutoscalerBuilder) Update(object client.Object) error {
	name := builder.Instance.ChildResourceName(builder.profile.Name, HorizontalPodAutoscalerSuffix)
	hpa := object.(*autoscalingv1.HorizontalPodAutoscaler)

	hpa.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)

	targetCPUUtilizationPercentage := int32(85)
	profileSpec := getProfileSpec(builder.profile.Name, builder.Instance)

	hpa.Spec.ScaleTargetRef = autoscalingv1.CrossVersionObjectReference{
		Kind:       "Deployment",
		Name:       name,
		APIVersion: "apps/v1",
	}
	hpa.Spec.MinReplicas = profileSpec.MinReplicas
	hpa.Spec.MaxReplicas = *profileSpec.MaxReplicas
	hpa.Spec.TargetCPUUtilizationPercentage = &targetCPUUtilizationPercentage

	if err := controllerutil.SetControllerReference(builder.Instance, hpa, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %v", err)
	}

	return nil
}

func (builder *HorizontalPodAutoscalerBuilder) ShouldDeploy(resources []runtime.Object) bool {
	return status.IsPersistentVolumeClaimBound(
		builder.Instance.ChildResourceName(builder.profile.Name, PersistentVolumeClaimSuffix),
		resources,
	) &&
		status.IsJobCompleted(
			builder.Instance.ChildResourceName(builder.profile.Name, JobSuffix),
			resources,
		)
}
