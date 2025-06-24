package resource

import (
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/metadata"
	"github.com/itayankri/OSRM-Operator/internal/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ConfigMapBuilder struct {
	ClusterScopedBuilder
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) ConfigMap(profiles []*osrmv1alpha1.ProfileSpec) *ConfigMapBuilder {
	return &ConfigMapBuilder{
		ClusterScopedBuilder{profiles},
		builder,
	}
}

func (builder *ConfigMapBuilder) Build() (client.Object, error) {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName(GatewaySuffix, ConfigMapSuffix),
			Namespace: builder.Instance.Namespace,
			Labels:    metadata.GetLabels(builder.Instance, metadata.ComponentLabelGateway),
		},
	}, nil
}

func (builder *ConfigMapBuilder) Update(object client.Object, siblings []runtime.Object) error {
	configMap := object.(*corev1.ConfigMap)
	configMap.ObjectMeta.Labels = metadata.GetLabels(builder.Instance, metadata.ComponentLabelGateway)

	if configMap.Data == nil {
		configMap.Data = make(map[string]string)
	}

	configMap.Data[nginxConfigurationTemplateName] = generateNginxConf(
		builder.Instance,
		builder.profiles,
		builder.Instance.Spec.Service.ExposingServices,
	)

	if err := controllerutil.SetControllerReference(builder.Instance, configMap, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %v", err)
	}

	return nil
}

func generateNginxConf(instance *osrmv1alpha1.OSRMCluster, profiles []*osrmv1alpha1.ProfileSpec, osrmServices []string) string {
	config := `
	events {
		worker_connections 2048;
	}
	http {
		large_client_header_buffers 4 128k;
		server {
			listen 80;
			server_name _;
			%s
		}
	}
	`
	locations := getNginxLocations(instance, profiles, osrmServices)
	return fmt.Sprintf(config, locations)
}

func getNginxLocations(instance *osrmv1alpha1.OSRMCluster, profiles []*osrmv1alpha1.ProfileSpec, osrmServices []string) string {
	var locations strings.Builder
	for _, profile := range profiles {
		for _, service := range osrmServices {
			location := formatNginxLocation(instance, *profile, service)
			locations.WriteString(location)
		}
	}

	return locations.String()
}

func formatNginxLocation(instance *osrmv1alpha1.OSRMCluster, profile osrmv1alpha1.ProfileSpec, osrmService string) string {
	internalPath := fmt.Sprintf("%s/v1/%s", osrmService, profile.GetInternalEndpoint())
	externalPath := fmt.Sprintf("%s/v1/%s", osrmService, profile.EndpointName)
	serviceName := instance.ChildResourceName(profile.Name, "")
	envVar := serviceToEnvVariable(serviceName)
	return fmt.Sprintf(`
			location /%s {
				proxy_pass http://${%s}/%s;
			}`, externalPath, envVar, internalPath)
}

func (builder *ConfigMapBuilder) ShouldDeploy(resources []runtime.Object) bool {
	for _, profile := range builder.Instance.Spec.Profiles {
		if !status.IsJobCompleted(builder.Instance.ChildResourceName(profile.Name, JobSuffix), resources) ||
			!status.IsPersistentVolumeClaimBound(builder.Instance.ChildResourceName(profile.Name, PersistentVolumeClaimSuffix), resources) {
			return false
		}
	}
	return true
}
