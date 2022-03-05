package resource

import (
	"fmt"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
)

func getProfileSpec(profile OSRMProfile, instance *osrmv1alpha1.OSRMCluster) *osrmv1alpha1.ProfileSpec {
	switch profile {
	case DrivingProfile:
		return instance.Spec.Profiles.Driving
	case CyclingProfile:
		return instance.Spec.Profiles.Cycling
	case FootProfile:
		return instance.Spec.Profiles.Foot
	default:
		panic(fmt.Sprintf("Profile %s is not supported", profile))
	}
}
