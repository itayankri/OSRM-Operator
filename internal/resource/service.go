package resource

import (
	"fmt"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type ServiceBuilder struct {
	ProfileScopedBuilder
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) Service(profile *osrmv1alpha1.ProfileSpec) *ServiceBuilder {
	return &ServiceBuilder{
		ProfileScopedBuilder{profile},
		builder,
	}
}

func (builder *ServiceBuilder) Build() (client.Object, error) {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", builder.Instance.Name, builder.profile.Name),
			Namespace: builder.Instance.Namespace,
		},
	}, nil
}

func (builder *ServiceBuilder) Update(object client.Object) error {
	name := fmt.Sprintf("%s-%s", builder.Instance.Name, builder.profile.Name)

	service := object.(*corev1.Service)

	service.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)

	service.Spec.Type = corev1.ServiceTypeClusterIP
	service.Spec.Ports = []corev1.ServicePort{
		{
			Name:     fmt.Sprintf("%s-port", name),
			Protocol: corev1.ProtocolTCP,
			Port:     80,
			TargetPort: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: 5000,
			},
		},
	}
	service.Spec.Selector = map[string]string{
		"app": name,
	}

	if err := controllerutil.SetControllerReference(builder.Instance, service, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %v", err)
	}

	return nil
}
