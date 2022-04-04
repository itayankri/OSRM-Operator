package resource

import (
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConfigMapBuilder struct {
	ClusterScopedBuilder
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) ConfigMap(profiles []osrmv1alpha1.ProfileSpec) *ConfigMapBuilder {
	return &ConfigMapBuilder{
		ClusterScopedBuilder{profiles},
		builder,
	}
}

func (builder *ConfigMapBuilder) Build() (client.Object, error) {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.Name,
			Namespace: builder.Instance.Namespace,
		},
	}, nil
}

func (builder *ConfigMapBuilder) Update(object client.Object) error {
	configMap := object.(*corev1.ConfigMap)
	configMap.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)

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

func generateNginxConf(instance *osrmv1alpha1.OSRMCluster, profiles []osrmv1alpha1.ProfileSpec, osrmServices []string) string {
	config := `
	events {
	
	}
	http {
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

func getNginxLocations(instance *osrmv1alpha1.OSRMCluster, profiles []osrmv1alpha1.ProfileSpec, osrmServices []string) string {
	var locations strings.Builder
	for _, profile := range profiles {
		for _, service := range osrmServices {
			location := formatNginxLocation(instance, profile, service)
			locations.WriteString(location)
		}
	}

	return locations.String()
}

func formatNginxLocation(instance *osrmv1alpha1.OSRMCluster, profile osrmv1alpha1.ProfileSpec, osrmService string) string {
	path := fmt.Sprintf("%s/v1/%s/", osrmService, profile.EndpointName)
	serviceName := fmt.Sprintf("%s-%s", instance.Name, profile.Name)
	envVar := serviceToEnvVariable(serviceName)
	return fmt.Sprintf(`
			location /%s {
				proxy_pass http://${%s};
			}`, path, envVar)
}
