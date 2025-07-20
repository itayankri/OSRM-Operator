package resource

import (
	"fmt"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/metadata"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type PodDisruptionBudgetBuilder struct {
	ProfileScopedBuilder
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) PodDisruptionBudget(profile *osrmv1alpha1.ProfileSpec) *PodDisruptionBudgetBuilder {
	return &PodDisruptionBudgetBuilder{
		ProfileScopedBuilder{profile},
		builder,
	}
}

func (builder *PodDisruptionBudgetBuilder) Build() (client.Object, error) {
	return &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName(builder.profile.Name, PodDisruptionBudgetSuffix),
			Namespace: builder.Instance.Namespace,
			Labels:    metadata.GetLabels(builder.Instance, metadata.ComponentLabelProfile),
		},
	}, nil
}

func (builder *PodDisruptionBudgetBuilder) Update(object client.Object, siblings []runtime.Object) error {
	name := builder.Instance.ChildResourceName(builder.profile.Name, PodDisruptionBudgetSuffix)
	pdb := object.(*policyv1.PodDisruptionBudget)
	pdb.ObjectMeta.Labels = metadata.GetLabels(builder.Instance, metadata.ComponentLabelProfile)
	pdb.Spec.MinAvailable = builder.profile.GetMinAvailable()
	pdb.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": name,
		},
	}

	if err := controllerutil.SetControllerReference(builder.Instance, pdb, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %v", err)
	}

	return nil
}
