package metadata

import (
	"strconv"
	"strings"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
)

type ComponentLabelValue string

const (
	ComponentLabelGateway ComponentLabelValue = "gateway"
	ComponentLabelProfile ComponentLabelValue = "profile"
)

const NameLabelKey = "app.kubernetes.io/name"
const PartOfLabelKey = "app.kubernetes.io/part-of"
const ComponentLabelKey = "app.kubernetes.io/component"
const GenerationLabelKey = "osrmcluster.itayankri/cluster-generation"
const EnvironmentLabelKey = "osrmcluster.itayankri/environment"

func GetLabels(instance *osrmv1alpha1.OSRMCluster, componentName ComponentLabelValue) map[string]string {
	return GetLabelsWithEnvironment(instance, componentName, "")
}

func GetLabelsWithEnvironment(instance *osrmv1alpha1.OSRMCluster, componentName ComponentLabelValue, environment string) map[string]string {
	labels := map[string]string{
		NameLabelKey:       instance.Name,
		PartOfLabelKey:     "osrmcluster",
		ComponentLabelKey:  string(componentName),
		GenerationLabelKey: strconv.FormatInt(instance.ObjectMeta.Generation, 10),
	}

	if environment != "" {
		labels[EnvironmentLabelKey] = environment
	}

	for label, value := range instance.Labels {
		if !strings.HasPrefix(label, "app.kubernetes.io") {
			labels[label] = value
		}
	}

	return labels
}
