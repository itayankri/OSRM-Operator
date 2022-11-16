package resource_test

import (
	"fmt"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/resource"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("Service builder", func() {
	var (
		scheme         *runtime.Scheme
		serviceBuilder resource.ResourceBuilder
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(osrmv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())
		serviceBuilder = osrmResourceBuilder.Service(instance.Spec.Profiles[0])
	})

	Context("Build", func() {
		It("Should use values from custom resource", func() {
			obj, err := serviceBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			service := obj.(*corev1.Service)

			By("generates a service object with the correct name and labels", func() {
				expectedName := fmt.Sprintf("%s-%s", instance.ObjectMeta.Name, instance.Spec.Profiles[0].Name)
				Expect(service.Name).To(Equal(expectedName))
			})

			By("generates a service object with the correct namespace", func() {
				Expect(service.Namespace).To(Equal(instance.Namespace))
			})
		})
	})

	Context("ShouldDeploy", func() {
		It("Should return 'false' when both PVC is bound and map builder Job is not completed yet", func() {
			resources := generateChildResources(false, false, instance.ObjectMeta.Name, instance.Spec.Profiles[0].Name)
			Expect(serviceBuilder.ShouldDeploy(resources)).To(Equal(false))
		})

		It("Should return 'false' when PVC is bound but map builder Job is not completed yet", func() {
			resources := generateChildResources(true, false, instance.ObjectMeta.Name, instance.Spec.Profiles[0].Name)
			Expect(serviceBuilder.ShouldDeploy(resources)).To(Equal(false))
		})

		It("Should return 'false' when PVC is not bound but map builder Job is completed", func() {
			resources := generateChildResources(false, true, instance.ObjectMeta.Name, instance.Spec.Profiles[0].Name)
			Expect(serviceBuilder.ShouldDeploy(resources)).To(Equal(false))
		})

		It("Should return 'true' when both PVC is bound and map builder Job is compoleted", func() {
			resources := generateChildResources(true, true, instance.ObjectMeta.Name, instance.Spec.Profiles[0].Name)
			Expect(serviceBuilder.ShouldDeploy(resources)).To(Equal(true))
		})
	})
})
