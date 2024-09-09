package resource_test

import (
	"fmt"
	"testing"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/resource"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

func generateChildResources(
	pvcBound bool,
	jobCompleted bool,
	instanceName string,
	profile string,
) []runtime.Object {
	pvcPhase := corev1.ClaimPending
	if pvcBound {
		pvcPhase = corev1.ClaimBound
	}

	jobConditionStatus := corev1.ConditionFalse
	if jobCompleted {
		jobConditionStatus = corev1.ConditionTrue
	}

	childResources := []runtime.Object{
		&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-%s-%s", instanceName, profile, resource.JobSuffix),
			},
			Status: batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{
					{
						Type:   batchv1.JobComplete,
						Status: jobConditionStatus,
					},
				},
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-%s", instanceName, profile),
			},
			Status: corev1.PersistentVolumeClaimStatus{
				Phase: pvcPhase,
			},
		},
	}

	return childResources
}

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
