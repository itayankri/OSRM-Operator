package resource

import (
	"fmt"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/metadata"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type JobBuilder struct {
	ProfileScopedBuilder
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) Job(profile *osrmv1alpha1.ProfileSpec) *JobBuilder {
	return &JobBuilder{
		ProfileScopedBuilder{profile},
		builder,
	}
}

func (builder *JobBuilder) Build() (client.Object, error) {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName(builder.profile.Name, JobSuffix),
			Namespace: builder.Instance.Namespace,
		},
	}, nil
}

func (builder *JobBuilder) Update(object client.Object, siblings []runtime.Object) error {
	job := object.(*batchv1.Job)

	job.Labels = metadata.GetLabels(builder.Instance, builder.Instance.Labels)

	job.Spec = batchv1.JobSpec{
		Selector: job.Spec.Selector,
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: job.Spec.Template.ObjectMeta.Labels,
			},
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyOnFailure,
				Containers: []corev1.Container{
					{
						Name:      builder.Instance.ChildResourceName(builder.profile.Name, JobSuffix),
						Image:     builder.Instance.Spec.MapBuilder.GetImage(),
						Resources: *builder.Instance.Spec.MapBuilder.GetResources(),
						Env: []corev1.EnvVar{
							{
								Name:  "ROOT_DIR",
								Value: osrmDataPath,
							},
							{
								Name:  "PARTITIONED_DATA_DIR",
								Value: osrmPartitionedData,
							},
							{
								Name:  "CUSTOMIZED_DATA_DIR",
								Value: osrmCustomizedData,
							},
							{
								Name:  "PBF_URL",
								Value: builder.Instance.Spec.PBFURL,
							},
							{
								Name:  "PROFILE",
								Value: builder.profile.GetProfile(),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      osrmDataVolumeName,
								MountPath: osrmDataPath,
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: osrmDataVolumeName,
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: builder.Instance.ChildResourceName(builder.profile.Name, PersistentVolumeClaimSuffix),
								ReadOnly:  false,
							},
						},
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(builder.Instance, job, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %v", err)
	}

	return nil
}

func (builder *JobBuilder) ShouldDeploy(resources []runtime.Object) bool {
	return true
}
