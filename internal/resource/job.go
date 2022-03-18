package resource

import (
	"fmt"
	"strings"

	"github.com/itayankri/OSRM-Operator/internal/metadata"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type JobBuilder struct {
	ProfileScopedBuilder
	*OSRMResourceBuilder
}

var profilesMap = map[OSRMProfile]string{
	DrivingProfile: "car.lua",
	CyclingProfile: "bicycle.lua",
	FootProfile:    "foot.lua",
}

func (builder *OSRMResourceBuilder) Job(profile OSRMProfile) *JobBuilder {
	return &JobBuilder{
		ProfileScopedBuilder{profile},
		builder,
	}
}

func (builder *JobBuilder) Build() (client.Object, error) {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%s", builder.Instance.Name, builder.profile, "map-builder"),
			Namespace: builder.Instance.Namespace,
		},
	}, nil
}

func (builder *JobBuilder) Update(object client.Object) error {
	name := fmt.Sprintf("%s-%s", builder.Instance.Name, builder.profile)
	pbfFileName := builder.Instance.Spec.GetPbfFileName()
	osrmFileName := strings.ReplaceAll(pbfFileName, "osm.pbf", "osrm")
	job := object.(*batchv1.Job)

	job.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)

	job.Spec = batchv1.JobSpec{
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyOnFailure,
				Containers: []corev1.Container{
					{
						Name:  fmt.Sprintf("%s-%s", name, "map-builder"),
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
								cd %s && \
								apt update && \
								apt --assume-yes install wget && \
								wget %s && \
								osrm-extract -p /opt/%s %s && \
								osrm-partition %s && \
								osrm-customize %s
							`,
								osrmDataPath,
								builder.Instance.Spec.PBFURL,
								profilesMap[builder.profile],
								pbfFileName,
								osrmFileName,
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
								ClaimName: name,
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
