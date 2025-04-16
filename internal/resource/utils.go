package resource

import (
	"encoding/json"
	"fmt"
	"strings"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/apis/core/validation"
)

func getProfileSpec(profileName string, instance *osrmv1alpha1.OSRMCluster) *osrmv1alpha1.ProfileSpec {
	for _, profile := range instance.Spec.Profiles {
		if profile.Name == profileName {
			return profile
		}
	}
	return nil
}

func serviceToEnvVariable(serviceName string) string {
	return fmt.Sprintf("%s_SERVICE_HOST", strings.ReplaceAll(strings.ToUpper(serviceName), "-", "_"))
}

func overridePodTemplateSpec(defaultPodTemplate *corev1.PodTemplateSpec, podTemplateOverride *runtime.RawExtension) error {
	defaultJSON, err := json.Marshal(defaultPodTemplate)
	if err != nil {
		return fmt.Errorf("failed to marshal default PodTemplateSpec: %v", err)
	}

	var overridePodTemplate corev1.PodTemplateSpec

	if err := json.Unmarshal(podTemplateOverride.Raw, &overridePodTemplate); err != nil {
		return fmt.Errorf("failed to decode podTemplateOverride: %v", err)
	}

	if errs := validation.ValidatePodTemplateSpec(&overridePodTemplate, field.NewPath("spec.podTemplateOverride")); len(errs) > 0 {
		return fmt.Errorf("invalid podTemplateOverride: %v", errs.ToAggregate())
	}

	overrideJSON, err := json.Marshal(overridePodTemplate)
	if err != nil {
		return fmt.Errorf("failed to marshal override PodTemplateSpec: %v", err)
	}

	mergedJSON, err := strategicpatch.StrategicMergePatch(defaultJSON, overrideJSON, corev1.PodTemplateSpec{})
	if err != nil {
		return fmt.Errorf("failed to merge PodTemplateSpec: %v", err)
	}

	// Unmarshal back to PodTemplateSpec
	if err := json.Unmarshal(mergedJSON, &defaultPodTemplate); err != nil {
		return fmt.Errorf("failed to unmarshal merged PodTemplateSpec: %v", err)
	}

	return nil
}
