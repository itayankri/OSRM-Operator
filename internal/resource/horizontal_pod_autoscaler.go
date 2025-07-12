package resource

import (
	"fmt"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/metadata"
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
			Labels:    metadata.GetLabels(builder.Instance, metadata.ComponentLabelProfile),
		},
	}, nil
}

func (builder *HorizontalPodAutoscalerBuilder) Update(object client.Object, siblings []runtime.Object) error {
	name := builder.Instance.ChildResourceName(builder.profile.Name, HorizontalPodAutoscalerSuffix)
	hpa := object.(*autoscalingv1.HorizontalPodAutoscaler)

	hpa.ObjectMeta.Labels = metadata.GetLabels(builder.Instance, metadata.ComponentLabelProfile)

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
