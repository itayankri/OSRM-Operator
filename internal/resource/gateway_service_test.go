package resource_test

import (
	"github.com/itayankri/OSRM-Operator/internal/resource"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("GatewayService builder", func() {
	Context("ShouldDeploy", func() {
		var builder resource.ResourceBuilder
		BeforeEach(func() {
			builder = osrmResourceBuilder.GatewayService(instance.Spec.Profiles)
		})

		It("Should return 'true' if at least one Profile is ready", func() {
			resources := []runtime.Object{}
			for _, profile := range instance.Spec.Profiles {
				resources = append(
					resources,
					generateChildResources(false, true, instance.Name, profile.Name)...,
				)
			}
			Expect(builder.ShouldDeploy(resources)).To(Equal(true))
		})

		It("Should return 'true' once all PVC's are bound and all Jobs completed", func() {
			resources := []runtime.Object{}
			for _, profile := range instance.Spec.Profiles {
				resources = append(
					resources,
					generateChildResources(true, true, instance.Name, profile.Name)...,
				)
			}
			Expect(builder.ShouldDeploy(resources)).To(Equal(true))
		})
	})
})
