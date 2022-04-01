package resource

import (
	"fmt"
	"strings"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
)

func getProfileSpec(profileName string, instance *osrmv1alpha1.OSRMCluster) *osrmv1alpha1.ProfileSpec {
	for _, profile := range instance.Spec.Profiles {
		if profile.Name == profileName {
			return &profile
		}
	}
	return nil
}

func serviceToEnvVariable(serviceName string) string {
	return fmt.Sprintf("%s_SERVICE_HOST", strings.ReplaceAll(strings.ToUpper(serviceName), "-", "_"))
}
