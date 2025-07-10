package resource

import (
	"fmt"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type PersistentVolumeClaimBuilder struct {
	ProfileScopedBuilder
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) PersistentVolumeClaim(profile *osrmv1alpha1.ProfileSpec) *PersistentVolumeClaimBuilder {
	return &PersistentVolumeClaimBuilder{
		ProfileScopedBuilder{profile},
		builder,
	}
}

func (builder *PersistentVolumeClaimBuilder) Build() (client.Object, error) {
	var name string
	var labels map[string]string
	
	if builder.Environment != "" {
		name = builder.Instance.ChildResourceNameWithEnvironment(builder.profile.Name, PersistentVolumeClaimSuffix, builder.Environment)
		labels = metadata.GetLabelsWithEnvironment(builder.Instance, metadata.ComponentLabelProfile, builder.Environment)
	} else {
		name = builder.Instance.ChildResourceName(builder.profile.Name, PersistentVolumeClaimSuffix)
		labels = metadata.GetLabels(builder.Instance, metadata.ComponentLabelProfile)
	}
	
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: builder.Instance.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				builder.Instance.Spec.Persistence.GetAccessMode(),
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: *builder.Instance.Spec.Persistence.Storage,
				},
			},
			VolumeName:       "",
			StorageClassName: &builder.Instance.Spec.Persistence.StorageClassName,
		},
	}, nil
}

func (builder *PersistentVolumeClaimBuilder) Update(object client.Object, siblings []runtime.Object) error {
	pvc := object.(*corev1.PersistentVolumeClaim)

	if builder.Environment != "" {
		pvc.ObjectMeta.Labels = metadata.GetLabelsWithEnvironment(builder.Instance, metadata.ComponentLabelProfile, builder.Environment)
	} else {
		pvc.ObjectMeta.Labels = metadata.GetLabels(builder.Instance, metadata.ComponentLabelProfile)
	}

	if err := controllerutil.SetControllerReference(builder.Instance, pvc, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %v", err)
	}

	return nil
}

func (*PersistentVolumeClaimBuilder) ShouldDeploy(resources []runtime.Object) bool {
	return true
}

func (builder *OSRMResourceBuilder) PersistentVolumeClaimWithEnvironment(profile *osrmv1alpha1.ProfileSpec) *PersistentVolumeClaimBuilder {
	return &PersistentVolumeClaimBuilder{
		ProfileScopedBuilder{profile},
		builder,
	}
}
