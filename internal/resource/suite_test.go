package resource_test

import (
	"testing"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/resource"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var osrmResourceBuilder *resource.OSRMResourceBuilder
var instance *osrmv1alpha1.OSRMCluster

func TestStatus(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resource Suite")
}

var _ = BeforeSuite(func() {
	instance = generateOSRMCluster()
	osrmResourceBuilder = &resource.OSRMResourceBuilder{
		Instance: instance,
	}
})

func generateOSRMCluster() *osrmv1alpha1.OSRMCluster {
	minReplicas := int32(2)
	maxReplicas := int32(4)
	storage := k8sresource.MustParse("100Mi")
	return &osrmv1alpha1.OSRMCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: osrmv1alpha1.OSRMClusterSpec{
			PBFURL: "https://download.geofabrik.de/asia/israel-and-palestine-latest.osm.pbf",
			Profiles: osrmv1alpha1.ProfilesSpec{
				{
					Name:         "car",
					EndpointName: "driving",
					MinReplicas:  &minReplicas,
					MaxReplicas:  &maxReplicas,
				},
			},
			Service: osrmv1alpha1.ServiceSpec{
				ExposingServices: []string{"route", "table"},
			},
			Persistence: osrmv1alpha1.PersistenceSpec{
				StorageClassName: "standard",
				Storage:          &storage,
			},
		},
	}
}
