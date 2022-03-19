package resource

import (
	"fmt"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/metadata"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type IngressBuilder struct {
	ClusterScopedBuilder
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) Ingress(profiles []OSRMProfile) *IngressBuilder {
	return &IngressBuilder{
		ClusterScopedBuilder{profiles},
		builder,
	}
}

func (builder *IngressBuilder) Build() (client.Object, error) {
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.Name,
			Namespace: builder.Instance.Namespace,
		},
	}, nil
}

func (builder *IngressBuilder) Update(object client.Object) error {
	ingress := object.(*networkingv1.Ingress)
	paths := []networkingv1.HTTPIngressPath{}
	services := []string{"route", "table"}

	if builder.Instance.Spec.Ingress.ExposingServices != nil {
		services = builder.Instance.Spec.Ingress.ExposingServices
	}

	for _, profile := range builder.profiles {
		for _, service := range services {
			path := builder.getIngressRulePath(profile, service)
			paths = append(paths, path)
		}
	}

	ingress.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)
	ingress.Annotations = annotationsFactory(*builder.Instance.Spec.Ingress)

	ingress.Spec = networkingv1.IngressSpec{
		IngressClassName: builder.Instance.Spec.Ingress.IngressClassName,
		Rules: []networkingv1.IngressRule{
			{
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: paths,
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(builder.Instance, ingress, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %v", err)
	}

	return nil
}

func (builder *IngressBuilder) getIngressRulePath(profile OSRMProfile, service string) networkingv1.HTTPIngressPath {
	serviceName := fmt.Sprintf("%s-%s", builder.Instance.Name, profile)

	path, pathType := pathFactory(profile, service, builder.Instance.Spec.Ingress.IngressClassName)

	return networkingv1.HTTPIngressPath{
		Path:     path,
		PathType: &pathType,
		Backend: networkingv1.IngressBackend{
			Service: &networkingv1.IngressServiceBackend{
				Name: serviceName,
				Port: networkingv1.ServiceBackendPort{
					Number: 80,
				},
			},
		},
	}
}

func pathFactory(profile OSRMProfile, service string, ingressClassName *string) (string, networkingv1.PathType) {
	if ingressClassName != nil {
		if *ingressClassName == "nginx" {
			return fmt.Sprintf("/%s/v1/%s/.*", service, profile), networkingv1.PathTypePrefix
		}
	}

	return fmt.Sprintf("/%s/v1/%s/*", service, profile), networkingv1.PathTypeImplementationSpecific
}

func annotationsFactory(ingressSpec osrmv1alpha1.IngressSpec) map[string]string {
	if ingressSpec.IngressClassName != nil {
		if *ingressSpec.IngressClassName == "nginx" {
			return map[string]string{
				"nginx.ingress.kubernetes.io/use-regex": "true",
			}
		}
	}

	return map[string]string{
		"kubernetes.io/ingress.global-static-ip-name": ingressSpec.IPName,
	}
}
