package resource

import (
	"fmt"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const osrmContainerName = "osrm-backend"
const defaultImage = "osrm/osrm-backend"
const finalizer = "ankri.io/osrm-operator"

type DeploymentBuilder struct {
	BaseBuilder
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) Deployment(profile OSRMProfile) *DeploymentBuilder {
	return &DeploymentBuilder{
		BaseBuilder{profile},
		builder,
	}
}

func (builder *DeploymentBuilder) Build() (client.Object, error) {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", builder.Instance.Name, builder.profile),
			Namespace: builder.Instance.Namespace,
		},
	}, nil
}

func (builder *DeploymentBuilder) Update(object client.Object) error {
	name := fmt.Sprintf("%s-%s", builder.Instance.Name, builder.profile)
	deployment := object.(*appsv1.Deployment)

	profileSpec := builder.getProfileSpec()
	ownerReferenceController := true

	deployment.Spec = appsv1.DeploymentSpec{
		Replicas: profileSpec.MinReplicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": name,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app": name,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						Controller: &ownerReferenceController,
					},
				},
				Finalizers: []string{finalizer},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  osrmContainerName,
						Image: builder.getImage(),
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: 5000,
							},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    k8sresource.Quantity{Format: "1"},
								corev1.ResourceMemory: k8sresource.Quantity{Format: "1Gi"},
							},
						},
					},
				},
			},
		},
	}

	return nil
}

func (builder *DeploymentBuilder) getProfileSpec() *osrmv1alpha1.ProfileSpec {
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

func (builder *DeploymentBuilder) getImage() string {
	if builder.Instance.Spec.Image != nil {
		return *builder.Instance.Spec.Image
	}
	return defaultImage
}
