package resource

import (
	"fmt"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/metadata"
	"github.com/itayankri/OSRM-Operator/internal/status"
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
			Name:      builder.Instance.Name,
			Namespace: builder.Instance.Namespace,
			Labels:    metadata.GetLabels(builder.Instance, "gateway"),
		},
	}, nil
}

func (builder *GatewayServiceBuilder) Update(object client.Object, siblings []runtime.Object) error {
	service := object.(*corev1.Service)

	service.ObjectMeta.Labels = metadata.GetLabels(builder.Instance, "gateway")

	service.Spec.Ports = []corev1.ServicePort{
		{
			Name:     fmt.Sprintf("%s-port", builder.Instance.Name),
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

	service.Spec.Type = builder.Instance.Spec.Service.GetType()
	builder.setAnnotations(service)

	if builder.Instance.Spec.Service.LoadBalancerIP != nil {
		service.Spec.LoadBalancerIP = *builder.Instance.Spec.Service.LoadBalancerIP
	}

	if err := controllerutil.SetControllerReference(builder.Instance, service, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %v", err)
	}

	return nil
}

func (builder *GatewayServiceBuilder) ShouldDeploy(resources []runtime.Object) bool {
	for _, profile := range builder.Instance.Spec.Profiles {
		if !status.IsJobCompleted(builder.Instance.ChildResourceName(profile.Name, JobSuffix), resources) ||
			!status.IsPersistentVolumeClaimBound(builder.Instance.ChildResourceName(profile.Name, PersistentVolumeClaimSuffix), resources) {
			return false
		}
	}
	return true
}

func (builder *GatewayServiceBuilder) setAnnotations(service *corev1.Service) {
	if builder.Instance.Spec.Service.Annotations != nil {
		service.Annotations = metadata.ReconcileAnnotations(service.Annotations, builder.Instance.Spec.Service.Annotations)
	}
}
