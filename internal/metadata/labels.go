package metadata

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ComponentLabelValue string

const (
	ComponentLabelGateway ComponentLabelValue = "gateway"
	ComponentLabelProfile ComponentLabelValue = "profile"
)

const NameLabelKey = "app.kubernetes.io/name"
const PartOfLabelKey = "app.kubernetes.io/part-of"
const ComponentLabelKey = "app.kubernetes.io/component"

func GetLabels(instance metav1.Object, componentName ComponentLabelValue) map[string]string {
	labels := map[string]string{
		NameLabelKey:      instance.GetName(),
		PartOfLabelKey:    "osrmcluster",
		ComponentLabelKey: string(componentName),
	}

	for label, value := range instance.GetLabels() {
		if !strings.HasPrefix(label, "app.kubernetes.io") {
			labels[label] = value
		}
	}

	return labels
}
