package resource

import (
	"fmt"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/metadata"
	"github.com/itayankri/OSRM-Operator/internal/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var gatewayDefaultReplicas = int32(2)

type GatewayDeploymentBuilder struct {
	ClusterScopedBuilder
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) GatewayDeployment(profiles []*osrmv1alpha1.ProfileSpec) *GatewayDeploymentBuilder {
	return &GatewayDeploymentBuilder{
		ClusterScopedBuilder{profiles},
		builder,
	}
}

func (builder *GatewayDeploymentBuilder) Build() (client.Object, error) {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.Name,
			Namespace: builder.Instance.Namespace,
		},
	}, nil
}

func (builder *GatewayDeploymentBuilder) Update(object client.Object) error {
	deployment := object.(*appsv1.Deployment)
	deployment.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)
	deployment.Spec = appsv1.DeploymentSpec{
		Replicas: &gatewayDefaultReplicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": fmt.Sprintf("%s-%s", builder.Instance.Name, GatewaySuffix),
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app": fmt.Sprintf("%s-%s", builder.Instance.Name, GatewaySuffix),
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  osrmContainerName,
						Image: gatewayImage,
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: 80,
							},
						},
						Resources: corev1.ResourceRequirements{
							Requests: map[corev1.ResourceName]resource.Quantity{
								"memory": resource.MustParse("500Mi"),
								"cpu":    resource.MustParse("0.5"),
							},
						},
						Command: []string{
							"/bin/sh",
							"-c",
						},
						Args: []string{`
								envsubst < /etc/nginx/nginx.tmpl > /etc/nginx.conf &&
								printenv &&
								cat /etc/nginx.conf &&
								nginx -g 'daemon off;' -c /etc/nginx.conf
							`,
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "nginx-conf",
								MountPath: "/etc/nginx",
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "nginx-conf",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: builder.Instance.Name,
								},
								Items: []corev1.KeyToPath{
									{
										Key:  nginxConfigurationTemplateName,
										Path: nginxConfigurationTemplateName,
									},
								},
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

func (*GatewayDeploymentBuilder) ShouldDeploy(resources []runtime.Object) bool {
	return status.IsPersistentVolumeClaimBound(resources) && status.IsJobCompleted(resources)
}
