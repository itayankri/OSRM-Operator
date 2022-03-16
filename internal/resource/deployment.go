package resource

import (
	"fmt"
	"strings"

	"github.com/itayankri/OSRM-Operator/internal/metadata"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const osrmContainerName = "osrm-backend"
const defaultImage = "osrm/osrm-backend"
const osrmDataVolumeName = "osrm-data"
const osrmDataPath = "/data"

type DeploymentBuilder struct {
	ProfileScopedBuilder
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) Deployment(profile OSRMProfile) *DeploymentBuilder {
	return &DeploymentBuilder{
		ProfileScopedBuilder{profile},
		builder,
	}
}

func (builder *DeploymentBuilder) Build() (client.Object, error) {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", builder.Instance.Name, builder.profile),
			Namespace: builder.Instance.Namespace,
			Labels:    metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels),
		},
	}, nil
}

func (builder *DeploymentBuilder) Update(object client.Object) error {
	name := fmt.Sprintf("%s-%s", builder.Instance.Name, builder.profile)
	deployment := object.(*appsv1.Deployment)
	pbfFileName := builder.Instance.Spec.GetPbfFileName()
	osrmFileName := strings.ReplaceAll(pbfFileName, "osm.pbf", "osrm")
	profileSpec := getProfileSpec(builder.profile, builder.Instance)

	deployment.Spec = appsv1.DeploymentSpec{
		Replicas: profileSpec.MinReplicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": name,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app": name,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  osrmContainerName,
						Image: builder.getImage(),
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: 5000,
							},
						},
						Resources: corev1.ResourceRequirements{
							Requests: map[corev1.ResourceName]resource.Quantity{
								"memory": resource.MustParse("1Gi"),
								"cpu":    resource.MustParse("1"),
							},
						},
						Command: []string{
							"osrm-routed",
							"--algorithm",
							"mld",
							fmt.Sprintf("%s/%s", osrmDataPath, osrmFileName),
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      osrmDataVolumeName,
								MountPath: osrmDataPath,
							},
						},
					},
				},
				InitContainers: []corev1.Container{
					{
						Name:  "map-builder",
						Image: builder.getImage(),
						Resources: corev1.ResourceRequirements{
							Requests: map[corev1.ResourceName]resource.Quantity{
								"memory": resource.MustParse("1Gi"),
								"cpu":    resource.MustParse("1"),
							},
						},
						Command: []string{
							fmt.Sprintf("[ \"$(ls -A %s)\" ]", osrmDataPath), "&&",
							"apt update", "&&",
							"apt --assume-yes install wget", "&&",
							fmt.Sprintf("wget %s", builder.Instance.Spec.PBFURL), "&&",
							fmt.Sprintf("osrm-extract -p car.lua %s", pbfFileName), "&&",
							fmt.Sprintf("osrm-partition %s", osrmFileName), "&&",
							fmt.Sprintf("osrm-customize %s", osrmFileName),
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
								ReadOnly:  true,
							},
						},
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(builder.Instance, deployment, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %v", err)
	}

	return nil
}

func (builder *DeploymentBuilder) getImage() string {
	if builder.Instance.Spec.Image != nil {
		return *builder.Instance.Spec.Image
	}
	return defaultImage
}
