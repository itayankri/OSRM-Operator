package resource

import (
	"fmt"
	"strings"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/status"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

func (builder *CronJobBuilder) Update(object client.Object) error {
	cronJob := object.(*batchv1.CronJob)
	pbfFileName := builder.Instance.Spec.GetPbfFileName()
	osrmFileName := strings.ReplaceAll(pbfFileName, "osm.pbf", "osrm")
	speedUpdatesFileName := builder.profile.SpeedUpdates.GetFileURL()

	cronJob.Spec = batchv1.CronJobSpec{
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
								Name:  builder.Instance.ChildResourceName(builder.profile.Name, CronJobSuffix),
								Image: builder.Instance.Spec.GetImage(),
								Resources: corev1.ResourceRequirements{
									Requests: map[corev1.ResourceName]resource.Quantity{
										"memory": resource.MustParse("100M"),
										"cpu":    resource.MustParse("100m"),
									},
								},
								Command: []string{
									"/bin/sh",
									"-c",
								},
								Args: []string{
									fmt.Sprintf(`
										apt update && \
										apt --assume-yes install curl && \
										cd %s/%s && \
										curl -O %s && \
										osrm-customize %s --segment-speed-file %s
									`,
										osrmDataPath,
										osrmCustomizedData,
										builder.profile.SpeedUpdates.URL,
										osrmFileName,
										speedUpdatesFileName,
									),
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
								Name: builder.Instance.Name,
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: builder.Instance.Name,
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
