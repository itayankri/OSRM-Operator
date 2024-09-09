package resource_test

import (
	"github.com/itayankri/OSRM-Operator/internal/resource"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("PersistentVolumeClaim builder", func() {
	Context("ShouldDeploy", func() {
		var builder resource.ResourceBuilder
		BeforeEach(func() {
			builder = osrmResourceBuilder.PersistentVolumeClaim(instance.Spec.Profiles[0])
		})

		It("Should always return 'true'", func() {
			resources := []runtime.Object{}
			Expect(builder.ShouldDeploy(resources)).To(Equal(true))
		})
	})
})
