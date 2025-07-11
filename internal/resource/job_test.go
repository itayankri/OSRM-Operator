package resource_test

import (
	"github.com/itayankri/OSRM-Operator/internal/resource"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Job builder", func() {
	Context("ShouldDeploy", func() {
		var builder resource.ResourceBuilder
		BeforeEach(func() {
			builder = osrmResourceBuilder.Job(instance.Spec.Profiles[0], "1")
		})

		It("Should always return 'true'", func() {
			resources := []runtime.Object{}
			Expect(builder.ShouldDeploy(resources)).To(Equal(true))
		})
	})
})
