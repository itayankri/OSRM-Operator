package resource

import (
	"encoding/json"
	"fmt"
	"strings"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes/scheme"
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

func ValidatePodTemplateSpec(jsonData []byte) (*corev1.PodTemplateSpec, error) {
	// Create a new PodTemplateSpec
	podTemplateSpec := &corev1.PodTemplateSpec{}

	// First try simple JSON unmarshaling to catch basic JSON syntax errors
	if err := json.Unmarshal(jsonData, podTemplateSpec); err != nil {
		return nil, fmt.Errorf("invalid JSON format: %w", err)
	}

	// For more thorough validation, use the Kubernetes decoder
	decoder := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode(jsonData, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid PodTemplateSpec: %w", err)
	}

	// Try to convert to PodTemplateSpec
	podTemplateSpec, ok := obj.(*corev1.PodTemplateSpec)
	if !ok {
		return nil, fmt.Errorf("JSON does not represent a valid PodTemplateSpec")
	}

	return podTemplateSpec, nil
}

func overridePodTemplateSpec(defaultPodTemplate *corev1.PodTemplateSpec, podTemplateOverride *runtime.RawExtension) error {
	defaultJSON, err := json.Marshal(defaultPodTemplate)
	if err != nil {
		return fmt.Errorf("failed to marshal default PodTemplateSpec: %v", err)
	}

	validatedPodTemplateSpecOverride, err := ValidatePodTemplateSpec(podTemplateOverride.Raw)
	if err != nil {
		return fmt.Errorf("invalid podTemplateOverride: %v", err)
	}

	overrideJSON, err := json.Marshal(validatedPodTemplateSpecOverride)
	if err != nil {
		return fmt.Errorf("failed to marshal override PodTemplateSpec: %v", err)
	}

	mergedJSON, err := strategicpatch.StrategicMergePatch(defaultJSON, overrideJSON, corev1.PodTemplateSpec{})
	if err != nil {
		return fmt.Errorf("failed to merge PodTemplateSpec: %v", err)
	}

	if err := json.Unmarshal(mergedJSON, &defaultPodTemplate); err != nil {
		return fmt.Errorf("failed to unmarshal merged PodTemplateSpec: %v", err)
	}

	return nil
}
