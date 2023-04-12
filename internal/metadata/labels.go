package metadata

import (
	"strconv"
	"strings"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
)

const NameLabel = "app.kubernetes.io/name"
const PartOfLabel = "app.kubernetes.io/part-of"
const GenerationLabel = "osrmcluster.itayankri/cluster-generation"

func GetLabels(instance *osrmv1alpha1.OSRMCluster, instanceLabels map[string]string) map[string]string {
	labels := map[string]string{
		NameLabel:       instance.Name,
		PartOfLabel:     "osrmcluster",
		GenerationLabel: strconv.FormatInt(instance.ObjectMeta.Generation, 10),
	}

	for label, value := range instanceLabels {
		if !strings.HasPrefix(label, "app.kubernetes.io") {
			labels[label] = value
		}
	}

	return labels
}
