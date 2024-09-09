package resource_test

import (
	"github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/resource"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CronJob builder", func() {
	Context("ShouldDeploy", func() {
		var builder resource.ResourceBuilder
		BeforeEach(func() {
			builder = osrmResourceBuilder.CronJob(instance.Spec.Profiles[0])
			instance.Spec.Profiles[0].SpeedUpdates = &v1alpha1.SpeedUpdatesSpec{
				Schedule: "30 * * * *",
			}
		})

		It("Should return 'false' if SpeedUpdates is not set, regardless to PVC and Job states", func() {
			instance.Spec.Profiles[0].SpeedUpdates = nil
			resources := generateChildResources(true, true, instance.Name, instance.Spec.Profiles[0].Name)
			Expect(builder.ShouldDeploy(resources)).To(Equal(false))
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
