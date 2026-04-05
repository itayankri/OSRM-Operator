package resource_test

import (
	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/resource"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("HorizontalPodAutoscaler builder", func() {
	Context("ResourceBuildersForPhase", func() {
		buildResourceBuilder := func(profiles osrmv1alpha1.ProfilesSpec) *resource.OSRMResourceBuilder {
			storage := k8sresource.MustParse("100Mi")
			return &resource.OSRMResourceBuilder{
				Instance: &osrmv1alpha1.OSRMCluster{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
					Spec: osrmv1alpha1.OSRMClusterSpec{
						PBFURL:   "https://example.com/map.osm.pbf",
						Profiles: profiles,
						Service:  osrmv1alpha1.ServiceSpec{ExposingServices: []string{"route"}},
						Persistence: osrmv1alpha1.PersistenceSpec{
							StorageClassName: "standard",
							Storage:          &storage,
						},
					},
				},
			}
		}

		containsHPA := func(builders []resource.ResourceBuilder) bool {
			for _, b := range builders {
				if _, ok := b.(*resource.HorizontalPodAutoscalerBuilder); ok {
					return true
				}
			}
			return false
		}

		It("should include HPA when both minReplicas and maxReplicas are set", func() {
			min, max := int32(1), int32(5)
			builder := buildResourceBuilder(osrmv1alpha1.ProfilesSpec{
				{Name: "car", EndpointName: "driving", MinReplicas: &min, MaxReplicas: &max},
			})

			deployingBuilders := builder.DeployingWorkersPhaseBuilders()
			deployedBuilders := builder.WorkersDeployedPhaseBuilders()

			Expect(containsHPA(deployingBuilders)).To(BeTrue())
			Expect(containsHPA(deployedBuilders)).To(BeTrue())
		})

		It("should not include HPA when maxReplicas is missing", func() {
			min := int32(1)
			builder := buildResourceBuilder(osrmv1alpha1.ProfilesSpec{
				{Name: "car", EndpointName: "driving", MinReplicas: &min},
			})

			deployingBuilders := builder.DeployingWorkersPhaseBuilders()
			deployedBuilders := builder.WorkersDeployedPhaseBuilders()

			Expect(containsHPA(deployingBuilders)).To(BeFalse())
			Expect(containsHPA(deployedBuilders)).To(BeFalse())
		})

		It("should not include HPA when minReplicas is missing", func() {
			max := int32(5)
			builder := buildResourceBuilder(osrmv1alpha1.ProfilesSpec{
				{Name: "car", EndpointName: "driving", MaxReplicas: &max},
			})

			deployingBuilders := builder.DeployingWorkersPhaseBuilders()
			deployedBuilders := builder.WorkersDeployedPhaseBuilders()

			Expect(containsHPA(deployingBuilders)).To(BeFalse())
			Expect(containsHPA(deployedBuilders)).To(BeFalse())
		})

		It("should not include HPA when neither minReplicas nor maxReplicas are set", func() {
			builder := buildResourceBuilder(osrmv1alpha1.ProfilesSpec{
				{Name: "base", EndpointName: "base"},
			})

			deployingBuilders := builder.DeployingWorkersPhaseBuilders()
			deployedBuilders := builder.WorkersDeployedPhaseBuilders()

			Expect(containsHPA(deployingBuilders)).To(BeFalse())
			Expect(containsHPA(deployedBuilders)).To(BeFalse())
		})
	})
})
