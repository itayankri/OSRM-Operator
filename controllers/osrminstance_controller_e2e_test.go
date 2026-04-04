package controllers_test

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	osrmResource "github.com/itayankri/OSRM-Operator/internal/resource"
	"github.com/itayankri/OSRM-Operator/internal/status"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var instanceE2eTestCounter int64

var _ = Describe("OSRMInstanceController End-to-End Tests", func() {
	Context("Full Workflow Tests", func() {
		var testInstance *osrmv1alpha1.OSRMInstance

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())

				Eventually(func() bool {
					instance := &osrmv1alpha1.OSRMInstance{}
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      testInstance.Name,
						Namespace: testInstance.Namespace,
					}, instance)
					return errors.IsNotFound(err)
				}, ClusterDeletionTimeout).Should(BeTrue())

				testInstance = nil
			}
		})

		It("should handle complete lifecycle: create → update PBF → delete", func() {
			testInstance = generateE2EOSRMInstance("full-lifecycle")

			By("Step 1: Creating the OSRMInstance")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForInstanceDeployment(ctx, testInstance)

			By("Step 1: Verifying PhaseWorkersDeployed")
			Eventually(func() osrmv1alpha1.Phase {
				instance := &osrmv1alpha1.OSRMInstance{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.Name,
					Namespace: testInstance.Namespace,
				}, instance)
				if err != nil {
					return ""
				}
				return instance.Status.Phase
			}, MapBuildingTimeout).Should(Equal(osrmv1alpha1.PhaseWorkersDeployed))

			By("Step 1: Verifying ReconciliationSuccess is True")
			Eventually(func() metav1.ConditionStatus {
				instance := &osrmv1alpha1.OSRMInstance{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.Name,
					Namespace: testInstance.Namespace,
				}, instance)
				if err != nil {
					return metav1.ConditionUnknown
				}
				for _, condition := range instance.Status.Conditions {
					if condition.Type == status.ConditionReconciliationSuccess {
						return condition.Status
					}
				}
				return metav1.ConditionUnknown
			}, MapBuildingTimeout).Should(Equal(metav1.ConditionTrue))

			By("Step 2: Updating PBFURL")
			Expect(updateOSRMInstanceWithRetry(testInstance, func(v *osrmv1alpha1.OSRMInstance) {
				v.Spec.PBFURL = "https://download.geofabrik.de/australia-oceania/marshall-islands-220101.osm.pbf"
			})).To(Succeed())

			By("Step 2: Waiting for PhaseWorkersRedeployed")
			Eventually(func() osrmv1alpha1.Phase {
				instance := &osrmv1alpha1.OSRMInstance{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.Name,
					Namespace: testInstance.Namespace,
				}, instance)
				if err != nil {
					return ""
				}
				return instance.Status.Phase
			}, MapBuildingTimeout).Should(Equal(osrmv1alpha1.PhaseWorkersRedeployed))

			By("Step 2: Verifying ReconciliationSuccess remains True after update")
			Eventually(func() metav1.ConditionStatus {
				instance := &osrmv1alpha1.OSRMInstance{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.Name,
					Namespace: testInstance.Namespace,
				}, instance)
				if err != nil {
					return metav1.ConditionUnknown
				}
				for _, condition := range instance.Status.Conditions {
					if condition.Type == status.ConditionReconciliationSuccess {
						return condition.Status
					}
				}
				return metav1.ConditionUnknown
			}, MapBuildingTimeout).Should(Equal(metav1.ConditionTrue))
		})

		It("should apply OSRMRoutedOptions flags to deployment command", func() {
			testInstance = generateE2EOSRMInstance("routed-options")
			maxTableSize := int32(200)
			testInstance.Spec.OSRMRoutedOptions = &osrmv1alpha1.OSRMRoutedOptions{
				MaxTableSize: &maxTableSize,
			}

			By("Creating the OSRMInstance with OSRMRoutedOptions")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForInstanceDeployment(ctx, testInstance)

			By("Verifying deployment command contains the routed flag")
			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      testInstance.ChildResourceName("", osrmResource.DeploymentSuffix),
				Namespace: testInstance.Namespace,
			}, deployment)).To(Succeed())

			args := deployment.Spec.Template.Spec.Containers[0].Args
			argsJoined := strings.Join(args, " ")
			Expect(argsJoined).To(ContainSubstring("--max-table-size 200"))
		})
	})

	Context("Failure Recovery Tests", func() {
		var testInstance *osrmv1alpha1.OSRMInstance

		BeforeEach(func() {
			testInstance = generateE2EOSRMInstance("failure-recovery")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForInstanceDeployment(ctx, testInstance)
		})

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())

				Eventually(func() bool {
					instance := &osrmv1alpha1.OSRMInstance{}
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      testInstance.Name,
						Namespace: testInstance.Namespace,
					}, instance)
					return errors.IsNotFound(err)
				}, ClusterDeletionTimeout).Should(BeTrue())

				testInstance = nil
			}
		})

		It("should recreate deleted deployment automatically", func() {
			By("Getting the existing deployment")
			oldDeployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      testInstance.ChildResourceName("", osrmResource.DeploymentSuffix),
				Namespace: testInstance.Namespace,
			}, oldDeployment)).To(Succeed())
			oldUID := oldDeployment.UID

			By("Deleting the deployment")
			Expect(k8sClient.Delete(ctx, oldDeployment)).To(Succeed())

			By("Verifying deployment is recreated with a new UID")
			Eventually(func() bool {
				newDeployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName("", osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, newDeployment)
				return err == nil && newDeployment.UID != oldUID
			}, 30*time.Second).Should(BeTrue())
		})
	})

	Context("Garbage Collection End-to-End Tests", func() {
		var testInstance *osrmv1alpha1.OSRMInstance

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())

				Eventually(func() bool {
					instance := &osrmv1alpha1.OSRMInstance{}
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      testInstance.Name,
						Namespace: testInstance.Namespace,
					}, instance)
					return errors.IsNotFound(err)
				}, ClusterDeletionTimeout).Should(BeTrue())

				testInstance = nil
			}
		})

		It("should complete PBF update without error conditions", func() {
			testInstance = generateE2EOSRMInstance("gc-pbf-update")

			By("Creating the OSRMInstance")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForInstanceDeployment(ctx, testInstance)

			By("Updating PBFURL")
			Expect(updateOSRMInstanceWithRetry(testInstance, func(v *osrmv1alpha1.OSRMInstance) {
				v.Spec.PBFURL = "https://download.geofabrik.de/australia-oceania/marshall-islands-220101.osm.pbf"
			})).To(Succeed())

			By("Waiting for redeployment to complete without errors")
			Eventually(func() osrmv1alpha1.Phase {
				instance := &osrmv1alpha1.OSRMInstance{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.Name,
					Namespace: testInstance.Namespace,
				}, instance)
				if err != nil {
					return ""
				}
				return instance.Status.Phase
			}, MapBuildingTimeout).Should(Equal(osrmv1alpha1.PhaseWorkersRedeployed))

			By("Verifying ReconciliationSuccess is True (no error condition)")
			Eventually(func() metav1.ConditionStatus {
				instance := &osrmv1alpha1.OSRMInstance{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.Name,
					Namespace: testInstance.Namespace,
				}, instance)
				if err != nil {
					return metav1.ConditionUnknown
				}
				for _, condition := range instance.Status.Conditions {
					if condition.Type == status.ConditionReconciliationSuccess {
						return condition.Status
					}
				}
				return metav1.ConditionUnknown
			}, MapBuildingTimeout).Should(Equal(metav1.ConditionTrue))
		})

		It("should clean up old generation PVC after redeployment", func() {
			testInstance = generateE2EOSRMInstance("gc-old-pvc")

			By("Creating the OSRMInstance")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForInstanceDeployment(ctx, testInstance)

			By("Triggering map generation update")
			Expect(updateOSRMInstanceWithRetry(testInstance, func(v *osrmv1alpha1.OSRMInstance) {
				v.Spec.PBFURL = "https://download.geofabrik.de/australia-oceania/marshall-islands-220101.osm.pbf"
			})).To(Succeed())

			By("Waiting for PhaseWorkersRedeployed")
			Eventually(func() osrmv1alpha1.Phase {
				instance := &osrmv1alpha1.OSRMInstance{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.Name,
					Namespace: testInstance.Namespace,
				}, instance)
				if err != nil {
					return ""
				}
				return instance.Status.Phase
			}, MapBuildingTimeout).Should(Equal(osrmv1alpha1.PhaseWorkersRedeployed))

			By("Verifying old generation PVC is cleaned up")
			Eventually(func() bool {
				pvc := &corev1.PersistentVolumeClaim{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName("", "1"),
					Namespace: testInstance.Namespace,
				}, pvc)
				return errors.IsNotFound(err) || pvc.DeletionTimestamp != nil
			}, 60*time.Second).Should(BeTrue())
		})
	})
})

// Helper functions for OSRMInstance e2e tests

func generateE2EOSRMInstance(name string) *osrmv1alpha1.OSRMInstance {
	stor := resource.MustParse("10Mi")
	accessMode := corev1.ReadWriteOnce
	minReplicas := int32(1)
	maxReplicas := int32(3)

	serialNumber := atomic.AddInt64(&instanceE2eTestCounter, 1)
	uniqueName := fmt.Sprintf("%s-%d", name, serialNumber)

	return &osrmv1alpha1.OSRMInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uniqueName,
			Namespace: "default",
		},
		Spec: osrmv1alpha1.OSRMInstanceSpec{
			PBFURL:      "https://download.geofabrik.de/australia-oceania/marshall-islands-210101.osm.pbf",
			Profile:     "car",
			MinReplicas: &minReplicas,
			MaxReplicas: &maxReplicas,
			Persistence: osrmv1alpha1.PersistenceSpec{
				StorageClassName: "standard",
				Storage:          &stor,
				AccessMode:       &accessMode,
			},
			Resources: &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("100Mi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("100Mi"),
				},
			},
		},
	}
}
