package resource

import (
	"fmt"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/metadata"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

func (builder *JobBuilder) Update(object client.Object) error {
	pbfFileName := builder.Instance.Spec.GetPbfFileName()
	osrmFileName := builder.Instance.Spec.GetOsrmFileName()
	job := object.(*batchv1.Job)

	job.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)

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
						Name:  builder.Instance.ChildResourceName(builder.profile.Name, JobSuffix),
						Image: builder.Instance.Spec.GetImage(),
						Resources: corev1.ResourceRequirements{
							Requests: map[corev1.ResourceName]resource.Quantity{
								"memory": resource.MustParse("1Gi"),
								"cpu":    resource.MustParse("1"),
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
								cd %s && \
								mkdir %s %s && \
								cd %s && \
								curl -O %s && \
								osrm-extract -p /opt/%s.lua %s && \
								osrm-partition %s && \
								cd ../%s && \
								rm -rf * &&\
								cp ../%s/* . && \
								osrm-customize %s
							`,
								osrmDataPath,
								osrmPartitionedData,
								osrmCustomizedData,
								osrmPartitionedData,
								builder.Instance.Spec.PBFURL,
								builder.profile.Name,
								pbfFileName,
								osrmFileName,
								osrmCustomizedData,
								osrmPartitionedData,
								osrmFileName,
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
