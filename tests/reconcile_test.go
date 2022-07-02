package tests_test

import (
	"context"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Context("Reconcile", func() {
	var cluster *osrmv1alpha1.OSRMCluster

	BeforeEach(func() {
		cluster = getOSRMClusterCR()
		Expect(client.Create(context.Background(), cluster)).To(Succeed())
	})

	Describe("Profile updates", func() {
		It("Should remove all resources that related to a certain profile after removing it from CR", func() {

		})

		It("Should update HPA when updating minReplicas", func() {

		})

		It("Should update HPA when updating minReplicas", func() {

		})

		It("Should recreate all resources that related to a certain profile after changing its name", func() {

		})

		It("Should update Nginx's config-map after changing endpointName", func() {

		})
	})

	Describe("Service updates", func() {
		It("Should update Nginx's config-map after changing exposingServices array", func() {

		})
	})

	Describe("Persisence updates", func() {
		It("Should create new pvcs and rerun map-builder jobs after changing storage", func() {

		})

		It("Should create new pvcs and rerun map-builder jobs after changing storageClassName", func() {

		})
	})
})
