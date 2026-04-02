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

type GatewayServiceBuilder struct {
	ClusterScopedBuilder
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) GatewayService(profiles []*osrmv1alpha1.ProfileSpec) *GatewayServiceBuilder {
	return &GatewayServiceBuilder{
		ClusterScopedBuilder{profiles},
		builder,
	}
}

func (builder *GatewayServiceBuilder) Build() (client.Object, error) {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.GetName(),
			Namespace: builder.Instance.GetNamespace(),
			Labels:    metadata.GetLabels(builder.Instance, metadata.ComponentLabelGateway),
		},
	}, nil
}

func (builder *GatewayServiceBuilder) Update(object client.Object, siblings []runtime.Object) error {
	service := object.(*corev1.Service)

	service.ObjectMeta.Labels = metadata.GetLabels(builder.Instance, metadata.ComponentLabelGateway)

	service.Spec.Ports = []corev1.ServicePort{
		{
			Name:     fmt.Sprintf("%s-port", builder.Instance.GetName()),
			Protocol: corev1.ProtocolTCP,
			Port:     80,
			TargetPort: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: 80,
			},
		},
	}
	service.Spec.Selector = map[string]string{
		"app": builder.Instance.ChildResourceName(GatewaySuffix, ServiceSuffix),
	}

	svc := builder.Instance.GetService()
	service.Spec.Type = svc.GetType()
	builder.setAnnotations(service)

	if svc.LoadBalancerIP != nil {
		service.Spec.LoadBalancerIP = *svc.LoadBalancerIP
	}

	if err := controllerutil.SetControllerReference(builder.Instance, service, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %v", err)
	}

	return nil
}

func (builder *GatewayServiceBuilder) setAnnotations(service *corev1.Service) {
	if builder.Instance.GetService().Annotations != nil {
		service.Annotations = metadata.ReconcileAnnotations(service.Annotations, builder.Instance.GetService().Annotations)
	}
}
