package tests_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ClusterCreationTimeout = 10 * time.Second
	ClusterDeletionTimeout = 5 * time.Second
)

func getOSRMClusterCR() *osrmv1alpha1.OSRMCluster {

	return &osrmv1alpha1.OSRMCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: osrmv1alpha1.OSRMClusterSpec{
			PBFURL: "",
			Profiles: osrmv1alpha1.ProfilesSpec{
				{
					Name:         "car",
					EndpointName: "driving",
					MinReplicas:  &[]int32{1}[0],
					MaxReplicas:  &[]int32{10}[0],
				},
				{
					Name:         "bicycle",
					EndpointName: "cycling",
					MinReplicas:  &[]int32{0}[0],
					MaxReplicas:  &[]int32{3}[0],
				},
			},
			Service: osrmv1alpha1.ServiceSpec{
				ExposingServices: []string{"route", "table", "match"},
			},
			Persistence: osrmv1alpha1.PersistenceSpec{
				Storage:          &resource.MustParse("100Mi"),
				StorageClassName: "Standard",
			},
		},
	}
}

func TestFunctional(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OSRM operator e2e test suite")
}
