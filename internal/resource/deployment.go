package resource

import (
	"fmt"
	"strings"
	"time"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/metadata"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type DeploymentBuilder struct {
	ProfileScopedBuilder
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) Deployment(profile *osrmv1alpha1.ProfileSpec) *DeploymentBuilder {
	return &DeploymentBuilder{
		ProfileScopedBuilder{profile},
		builder,
	}
}

func (builder *DeploymentBuilder) Build() (client.Object, error) {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName(builder.profile.Name, DeploymentSuffix),
			Namespace: builder.Instance.Namespace,
			Labels:    metadata.GetLabels(builder.Instance, metadata.ComponentLabelProfile),
		},
	}, nil
}

func (builder *DeploymentBuilder) Update(object client.Object, siblings []runtime.Object) error {
	name := builder.Instance.ChildResourceName(builder.profile.Name, DeploymentSuffix)
	deployment := object.(*appsv1.Deployment)
	pbfFileName := builder.Instance.Spec.GetPbfFileName()
	osrmFileName := strings.ReplaceAll(pbfFileName, "osm.pbf", "osrm")
	labelSelector := map[string]string{
		"app": name,
	}

	deployment.ObjectMeta.Labels = metadata.GetLabels(builder.Instance, metadata.ComponentLabelProfile)
	deployment.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: labelSelector,
	}

	deployment.Spec.Template = corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: labelSelector,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  osrmContainerName,
					Image: builder.Instance.Spec.GetImage(),
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: containerPort,
						},
					},
					Resources: *builder.profile.GetResources(),
					Command: []string{
						"/bin/sh",
						"-c",
					},
					Args: []string{
						fmt.Sprintf(`
							cd %s/%s && \
							osrm-routed %s --algorithm mld --max-matching-size 21474836
						`,
							osrmDataPath,
							osrmCustomizedData,
							osrmFileName,
						),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      osrmDataVolumeName,
							MountPath: osrmDataPath,
						},
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path:   fmt.Sprintf("/nearest/v1/%s/34.761122,32.051346", builder.profile.Name),
								Port:   intstr.IntOrString{IntVal: containerPort},
								Scheme: corev1.URISchemeHTTP,
							},
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
							ReadOnly:  true,
						},
					},
				},
			},
		},
	}

	builder.setAnnotations(deployment, siblings)

	if err := controllerutil.SetControllerReference(builder.Instance, deployment, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %v", err)
	}

	return nil
}

func (builder *DeploymentBuilder) setAnnotations(deployment *appsv1.Deployment, siblings []runtime.Object) {
	for _, resource := range siblings {
		if cron, ok := resource.(*batchv1.CronJob); ok {
			if cron.ObjectMeta.Name == builder.Instance.ChildResourceName(builder.profile.Name, CronJobSuffix) {
				if cron.Status.LastSuccessfulTime != nil {
					if deployment.Spec.Template.ObjectMeta.Annotations == nil {
						deployment.Spec.Template.ObjectMeta.Annotations = map[string]string{}
					}
					deployment.Spec.Template.ObjectMeta.Annotations[LastTrafficUpdateTimeAnnotation] = cron.Status.LastSuccessfulTime.Format(time.RFC3339)
				}
			}
		}
	}
}
