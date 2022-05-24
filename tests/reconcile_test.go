package tests_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Context("Reconcile", func() {
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
