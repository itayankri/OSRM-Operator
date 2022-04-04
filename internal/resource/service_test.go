package resource_test

import (
	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/resource"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
)

func generateOSRMCluster() osrmv1alpha1.OSRMCluster {
	minReplicas := int32(2)
	maxReplicas := int32(4)
	storage := k8sresource.MustParse("100Mi")
	return osrmv1alpha1.OSRMCluster{
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

var _ = Context("Services", func() {
	var (
		instance osrmv1alpha1.OSRMCluster
		builder  resource.OSRMResourceBuilder
		scheme   *runtime.Scheme
	)

	Describe("Build", func() {
		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(osrmv1alpha1.AddToScheme(scheme)).To(Succeed())
			Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())
			instance = generateOSRMCluster()
			builder = resource.OSRMResourceBuilder{
				Instance: &instance,
				Scheme:   scheme,
			}
		})

		It("Should use values from custom resource", func() {
			serviceBuilder := builder.Service(instance.Spec.Profiles[0])
			obj, err := serviceBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			service := obj.(*corev1.Service)

			By("generates a service object with the correct name and labels", func() {
				expectedName := instance.Name
				Expect(service.Name).To(Equal(expectedName))
			})

			By("generates a service object with the correct namespace", func() {
				Expect(service.Namespace).To(Equal(instance.Namespace))
			})
		})
	})
})
