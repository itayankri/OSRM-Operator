package resource

import (
	"fmt"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type HorizontalPodAutoscalerBuilder struct {
	BaseBuilder
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) HorizontalPodAutoscaler(profile OSRMProfile) *HorizontalPodAutoscalerBuilder {
	return &HorizontalPodAutoscalerBuilder{
		BaseBuilder{profile},
		builder,
	}
}

func (builder *HorizontalPodAutoscalerBuilder) Build() (client.Object, error) {
	return &autoscalingv1.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", builder.Instance.Name, builder.profile),
			Namespace: builder.Instance.Namespace,
		},
	}, nil
}

func (builder *HorizontalPodAutoscalerBuilder) Update(object client.Object) error {
	name := fmt.Sprintf("%s-%s", builder.Instance.Name, builder.profile)
	hpa := object.(*autoscalingv1.HorizontalPodAutoscaler)

	targetCPUUtilizationPercentage := int32(85)
	profileSpec := builder.getProfileSpec()

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

func (builder *HorizontalPodAutoscalerBuilder) getProfileSpec() *osrmv1alpha1.ProfileSpec {
	switch builder.BaseBuilder.profile {
	case DrivingProfile:
		return builder.Instance.Spec.Profiles.Driving
	case CyclingProfile:
		return builder.Instance.Spec.Profiles.Cycling
	case FootProfile:
		return builder.Instance.Spec.Profiles.Foot
	default:
		panic(fmt.Sprintf("Profile %s is not supported", builder.BaseBuilder.profile))
	}
}
