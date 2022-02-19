package resource

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceBuilder struct {
	profile string
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) Service(profile OSRMProfile) *ServiceBuilder {
	return &ServiceBuilder{
		BaseBuilder{profile},
		builder,
	}
}

func (builder *ServiceBuilder) Build() (client.Object, error) {
	baseName := fmt.Sprintf("%s-%s", builder.Instance.Name, builder.profile)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      baseName,
			Namespace: builder.Instance.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				corev1.ServicePort{
					Name:     fmt.Sprintf("%s-port", baseName),
					Protocol: corev1.ProtocolTCP,
					Port:     80,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 5000,
					},
				},
			},
			Selector: map[string]string{
				"app": baseName,
			},
		},
	}, nil
}

func (builder *ServiceBuilder) Update(object client.Object) error {
	service := object.(*corev1.Service)
	service.Spec.Type = corev1.ServiceTypeClusterIP
	service.Spec.Selector = 
	return nil
}
