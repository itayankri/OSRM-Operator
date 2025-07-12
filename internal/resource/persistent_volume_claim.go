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
	MapGenerationScopedBuilder
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) PersistentVolumeClaim(profile *osrmv1alpha1.ProfileSpec, mapGeneration string) *PersistentVolumeClaimBuilder {
	return &PersistentVolumeClaimBuilder{
		ProfileScopedBuilder{profile},
		MapGenerationScopedBuilder{generation: mapGeneration},
		builder,
	}
}

func (builder *PersistentVolumeClaimBuilder) Build() (client.Object, error) {
	name := builder.Instance.ChildResourceName(builder.profile.Name, builder.MapGenerationScopedBuilder.generation)
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: builder.Instance.Namespace,
			Labels:    metadata.GetLabels(builder.Instance, metadata.ComponentLabelProfile),
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

	pvc.ObjectMeta.Labels = metadata.GetLabels(builder.Instance, metadata.ComponentLabelProfile)

	builder.setAnnotations(pvc)

	if err := controllerutil.SetControllerReference(builder.Instance, pvc, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %v", err)
	}

	return nil
}

func (builder *PersistentVolumeClaimBuilder) setAnnotations(pvc *corev1.PersistentVolumeClaim) {
	if builder.Instance.Spec.Service.Annotations != nil {
		pvc.Annotations = metadata.ReconcileAnnotations(pvc.Annotations, map[string]string{metadata.MapGenerationAnnotation: builder.MapGeneration})
	}
}
