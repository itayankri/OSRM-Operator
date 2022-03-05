package resource

import (
	"fmt"

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

	for _, profile := range builder.profiles {
		rule := builder.getIngressRule(profile)
		rules = append(rules, *rule)
	}

	ingress.Spec = networkingv1.IngressSpec{
		Rules: rules,
	}

	if err := controllerutil.SetControllerReference(builder.Instance, ingress, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %v", err)
	}

	return nil
}

func (builder *IngressBuilder) getIngressRule(profile OSRMProfile) *networkingv1.IngressRule {
	serviceName := fmt.Sprintf("%s-%s", builder.Instance.Name, profile)
	pathType := networkingv1.PathTypeImplementationSpecific
	return &networkingv1.IngressRule{
		IngressRuleValue: networkingv1.IngressRuleValue{
			HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{
					{
						Path:     fmt.Sprintf("/route/v1/%s/*", profile),
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
