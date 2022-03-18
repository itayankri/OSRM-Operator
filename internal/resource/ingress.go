package resource

import (
	"fmt"

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
	rules := []networkingv1.IngressRule{}
	services := []string{"route", "table"}

	if builder.Instance.Spec.Ingress != nil && builder.Instance.Spec.Ingress.ExposingServices != nil {
		services = builder.Instance.Spec.Ingress.ExposingServices
	}

	for _, profile := range builder.profiles {
		for _, service := range services {
			rule := builder.getIngressRule(profile, service)
			rules = append(rules, *rule)
		}
	}

	ingress.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)

	ingress.Spec = networkingv1.IngressSpec{
		IngressClassName: builder.Instance.Spec.Ingress.IngressClassName,
		Rules:            rules,
	}

	if err := controllerutil.SetControllerReference(builder.Instance, ingress, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %v", err)
	}

	return nil
}

func (builder *IngressBuilder) getIngressRule(profile OSRMProfile, service string) *networkingv1.IngressRule {
	serviceName := fmt.Sprintf("%s-%s", builder.Instance.Name, profile)
	var path string
	var pathType networkingv1.PathType

	if builder.Instance.Spec.Ingress != nil &&
		builder.Instance.Spec.Ingress.IngressClassName != nil &&
		*builder.Instance.Spec.Ingress.IngressClassName == "nginx" {
		pathType = networkingv1.PathTypePrefix
		path = fmt.Sprintf("/%s/v1/%s/.*", service, profile)
	} else {
		pathType = networkingv1.PathTypeImplementationSpecific
		path = fmt.Sprintf("/%s/v1/%s/*", service, profile)
	}

	return &networkingv1.IngressRule{
		IngressRuleValue: networkingv1.IngressRuleValue{
			HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{
					{
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
					},
				},
			},
		},
	}
}
