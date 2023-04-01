package resource

import (
	"fmt"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/status"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type CronJobBuilder struct {
	ProfileScopedBuilder
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) CronJob(profile *osrmv1alpha1.ProfileSpec) *CronJobBuilder {
	return &CronJobBuilder{
		ProfileScopedBuilder{profile},
		builder,
	}
}

func (builder *CronJobBuilder) Build() (client.Object, error) {
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName(builder.profile.Name, CronJobSuffix),
			Namespace: builder.Instance.Namespace,
		},
	}, nil
}

func (builder *CronJobBuilder) Update(object client.Object, siblings []runtime.Object) error {
	cronJob := object.(*batchv1.CronJob)

	cronJob.Spec = batchv1.CronJobSpec{
		Suspend:  builder.profile.SpeedUpdates.Suspend,
		Schedule: builder.profile.SpeedUpdates.Schedule,
		JobTemplate: batchv1.JobTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name:      builder.Instance.ChildResourceName(builder.profile.Name, CronJobSuffix),
				Namespace: builder.Instance.Namespace,
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyOnFailure,
						Containers: []corev1.Container{
							{
								Name:      builder.Instance.ChildResourceName(builder.profile.Name, CronJobSuffix),
								Image:     builder.profile.GetSpeedUpdatesImage(),
								Resources: *builder.profile.SpeedUpdates.GetResources(),
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
										Name:  "URL",
										Value: builder.profile.SpeedUpdates.URL,
									},
									{
										Name:  "OSRM_FILE_NAME",
										Value: builder.Instance.Spec.GetOsrmFileName(),
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
			},
		},
	}

	if err := controllerutil.SetControllerReference(builder.Instance, cronJob, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %v", err)
	}

	return nil
}

func (builder *CronJobBuilder) ShouldDeploy(resources []runtime.Object) bool {
	return builder.profile.SpeedUpdates != nil &&
		status.IsPersistentVolumeClaimBound(
			builder.Instance.ChildResourceName(builder.profile.Name, PersistentVolumeClaimSuffix),
			resources,
		) &&
		status.IsJobCompleted(
			builder.Instance.ChildResourceName(builder.profile.Name, JobSuffix),
			resources,
		)
}
