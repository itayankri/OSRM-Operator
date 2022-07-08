package tests_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var _ = Context("Reconcile", func() {
	var cluster *osrmv1alpha1.OSRMCluster
	var k client.Client

	BeforeEach(func() {
		k, _ = client.New(config.GetConfigOrDie(), client.Options{})
		cluster = getOSRMClusterCR()
		Expect(k.Create(context.Background(), cluster)).To(Succeed())
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
