package resource

import (
	"fmt"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
			Name:      builder.Instance.ChildResourceName(builder.profile.Name, ServiceSuffix),
			Namespace: builder.Instance.Namespace,
			Labels:    metadata.GetLabels(builder.Instance, metadata.ComponentLabelProfile),
		},
	}, nil
}

func (builder *ServiceBuilder) Update(object client.Object, siblings []runtime.Object) error {
	name := builder.Instance.ChildResourceName(builder.profile.Name, ServiceSuffix)

	service := object.(*corev1.Service)

	service.ObjectMeta.Labels = metadata.GetLabels(builder.Instance, metadata.ComponentLabelProfile)

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
