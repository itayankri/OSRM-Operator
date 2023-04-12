package metadata

import (
	"strconv"
	"strings"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
)

const GenerationLabel = "osrmcluster.itayankri/generation"

func GetLabels(instance *osrmv1alpha1.OSRMCluster, instanceLabels map[string]string) map[string]string {
	labels := map[string]string{
		"app.kubernetes.io/name":    instance.Name,
		"app.kubernetes.io/part-of": "osrmcluster",
		GenerationLabel:             strconv.FormatInt(instance.ObjectMeta.Generation, 10),
	}

	for label, value := range instanceLabels {
		if !strings.HasPrefix(label, "app.kubernetes.io") {
			labels[label] = value
		}
	}

	return labels
}
