package resource

import (
	"fmt"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/status"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
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
	return &policyv1beta1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName(builder.profile.Name, PodDisruptionBudgetSuffix),
			Namespace: builder.Instance.Namespace,
		},
	}, nil
}

func (builder *PodDisruptionBudgetBuilder) Update(object client.Object) error {
	name := builder.Instance.ChildResourceName(builder.profile.Name, PodDisruptionBudgetSuffix)
	pdb := object.(*policyv1beta1.PodDisruptionBudget)

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

func (builder *PodDisruptionBudgetBuilder) ShouldDeploy(resources []runtime.Object) bool {
	return status.IsPersistentVolumeClaimBound(
		builder.Instance.ChildResourceName(builder.profile.Name, PersistentVolumeClaimSuffix),
		resources,
	) &&
		status.IsJobCompleted(
			builder.Instance.ChildResourceName(builder.profile.Name, JobSuffix),
			resources,
		)
}
