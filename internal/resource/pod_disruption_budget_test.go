package resource_test

import (
	"github.com/itayankri/OSRM-Operator/internal/resource"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PodDisruptionBudget builder", func() {
	Context("ShouldDeploy", func() {
		var builder resource.ResourceBuilder
		BeforeEach(func() {
			builder = osrmResourceBuilder.PodDisruptionBudget(instance.Spec.Profiles[0])
		})

		It("Should return 'false' when both PVC is bound and map builder Job is not completed yet", func() {
			resources := generateChildResources(false, false, instance.Name, instance.Spec.Profiles[0].Name)
			Expect(builder.ShouldDeploy(resources)).To(Equal(false))
		})

		It("Should return 'false' when PVC is bound but map builder Job is not completed yet", func() {
			resources := generateChildResources(true, false, instance.Name, instance.Spec.Profiles[0].Name)
			Expect(builder.ShouldDeploy(resources)).To(Equal(false))
		})

		It("Should return 'false' when PVC is not bound but map builder Job is completed", func() {
			resources := generateChildResources(false, true, instance.Name, instance.Spec.Profiles[0].Name)
			Expect(builder.ShouldDeploy(resources)).To(Equal(false))
		})

		It("Should return 'true' when both PVC is bound and map builder Job is compoleted", func() {
			resources := generateChildResources(true, true, instance.Name, instance.Spec.Profiles[0].Name)
			Expect(builder.ShouldDeploy(resources)).To(Equal(true))
		})
	})
})
