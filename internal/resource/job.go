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
	var name string
	var labels map[string]string
	
	if builder.Environment != "" {
		name = builder.Instance.ChildResourceNameWithEnvironment(builder.profile.Name, JobSuffix, builder.Environment)
		labels = metadata.GetLabelsWithEnvironment(builder.Instance, metadata.ComponentLabelProfile, builder.Environment)
	} else {
		name = builder.Instance.ChildResourceName(builder.profile.Name, JobSuffix)
		labels = metadata.GetLabels(builder.Instance, metadata.ComponentLabelProfile)
	}
	
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: builder.Instance.Namespace,
			Labels:    labels,
		},
	}, nil
}

func (builder *JobBuilder) Update(object client.Object, siblings []runtime.Object) error {
	job := object.(*batchv1.Job)

	if builder.Environment != "" {
		job.ObjectMeta.Labels = metadata.GetLabelsWithEnvironment(builder.Instance, metadata.ComponentLabelProfile, builder.Environment)
	} else {
		job.ObjectMeta.Labels = metadata.GetLabels(builder.Instance, metadata.ComponentLabelProfile)
	}

	env := []corev1.EnvVar{
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
	}

	if builder.Instance.Spec.MapBuilder.ExtractOptions != nil {
		env = append(env, corev1.EnvVar{
			Name:  "EXTRACT_OPTIONS",
			Value: *builder.Instance.Spec.MapBuilder.ExtractOptions,
		})
	}

	if builder.Instance.Spec.MapBuilder.PartitionOptions != nil {
		env = append(env, corev1.EnvVar{
			Name:  "PARTITION_OPTIONS",
			Value: *builder.Instance.Spec.MapBuilder.PartitionOptions,
		})
	}

	if builder.Instance.Spec.MapBuilder.CustomizeOptions != nil {
		env = append(env, corev1.EnvVar{
			Name:  "CUSTOMIZE_OPTIONS",
			Value: *builder.Instance.Spec.MapBuilder.CustomizeOptions,
		})
	}

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
						Name:      builder.getJobContainerName(),
						Image:     builder.Instance.Spec.MapBuilder.GetImage(),
						Resources: *builder.Instance.Spec.MapBuilder.GetResources(),
						Env:       env,
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
								ClaimName: builder.getPvcName(),
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

func (builder *JobBuilder) getJobContainerName() string {
	if builder.Environment != "" {
		return builder.Instance.ChildResourceNameWithEnvironment(builder.profile.Name, JobSuffix, builder.Environment)
	}
	return builder.Instance.ChildResourceName(builder.profile.Name, JobSuffix)
}

func (builder *JobBuilder) getPvcName() string {
	if builder.Environment != "" {
		return builder.Instance.ChildResourceNameWithEnvironment(builder.profile.Name, PersistentVolumeClaimSuffix, builder.Environment)
	}
	return builder.Instance.ChildResourceName(builder.profile.Name, PersistentVolumeClaimSuffix)
}

func (builder *OSRMResourceBuilder) JobWithEnvironment(profile *osrmv1alpha1.ProfileSpec) *JobBuilder {
	return &JobBuilder{
		ProfileScopedBuilder{profile},
		builder,
	}
}
