package controllers_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	osrmResource "github.com/itayankri/OSRM-Operator/internal/resource"
	"github.com/itayankri/OSRM-Operator/internal/status"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var instanceIntegrationTestCounter int64

var _ = Describe("OSRMInstanceController Integration Tests", func() {
	Context("Basic Lifecycle Tests", func() {
		var testInstance *osrmv1alpha1.OSRMInstance

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())
				testInstance = nil
			}
		})

		It("should create instance and progress through phases to WorkersDeployed", func() {
			testInstance = generateOSRMInstance("lifecycle-test")

			By("Creating the OSRMInstance")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())

			By("Verifying the instance progresses through phases")
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
			}, MapBuildingTimeout).Should(Or(
				Equal(osrmv1alpha1.PhaseBuildingMap),
				Equal(osrmv1alpha1.PhaseDeployingWorkers),
				Equal(osrmv1alpha1.PhaseWorkersDeployed),
			))

			By("Verifying deployment and service are created")
			Eventually(func() error {
				deployment := &appsv1.Deployment{}
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName("", osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, deployment); err != nil {
					return err
				}

				service := &corev1.Service{}
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName("", osrmResource.ServiceSuffix),
					Namespace: testInstance.Namespace,
				}, service)
			}, MapBuildingTimeout).Should(Succeed())
		})

		It("should create HPA and PDB when min/maxReplicas are set", func() {
			testInstance = generateOSRMInstance("hpa-pdb-test")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForInstanceDeployment(ctx, testInstance)

			By("Verifying HPA exists")
			hpa := &autoscalingv1.HorizontalPodAutoscaler{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      testInstance.ChildResourceName("", osrmResource.HorizontalPodAutoscalerSuffix),
				Namespace: testInstance.Namespace,
			}, hpa)).To(Succeed())

			By("Verifying PDB exists")
			pdb := &policyv1.PodDisruptionBudget{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      testInstance.ChildResourceName("", osrmResource.PodDisruptionBudgetSuffix),
				Namespace: testInstance.Namespace,
			}, pdb)).To(Succeed())
		})
	})

	Context("Map Update Tests", func() {
		var testInstance *osrmv1alpha1.OSRMInstance

		BeforeEach(func() {
			testInstance = generateOSRMInstance("map-update")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForInstanceDeployment(ctx, testInstance)
		})

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())
				testInstance = nil
			}
		})

		It("should trigger PhaseUpdatingMap when PBFURL changes", func() {
			By("Updating the PBFURL")
			Expect(updateOSRMInstanceWithRetry(testInstance, func(v *osrmv1alpha1.OSRMInstance) {
				v.Spec.PBFURL = "https://download.geofabrik.de/australia-oceania/marshall-islands-220101.osm.pbf"
			})).To(Succeed())

			By("Verifying phase transitions through update phases")
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
			}, MapBuildingTimeout).Should(Or(
				Equal(osrmv1alpha1.PhaseUpdatingMap),
				Equal(osrmv1alpha1.PhaseRedepoloyingWorkers),
				Equal(osrmv1alpha1.PhaseWorkersRedeployed),
			))
		})

		It("should create generation-2 PVC during map update", func() {
			By("Updating the PBFURL")
			Expect(updateOSRMInstanceWithRetry(testInstance, func(v *osrmv1alpha1.OSRMInstance) {
				v.Spec.PBFURL = "https://download.geofabrik.de/australia-oceania/marshall-islands-220101.osm.pbf"
			})).To(Succeed())

			By("Verifying generation-2 PVC is created")
			Eventually(func() error {
				pvc := &corev1.PersistentVolumeClaim{}
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName("", "2"),
					Namespace: testInstance.Namespace,
				}, pvc)
			}, MapBuildingTimeout).Should(Succeed())
		})
	})

	Context("SpeedUpdates CronJob Tests", func() {
		var testInstance *osrmv1alpha1.OSRMInstance

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())
				testInstance = nil
			}
		})

		It("should create CronJob when SpeedUpdates is configured", func() {
			testInstance = generateOSRMInstance("speed-updates-test")
			testInstance.Spec.SpeedUpdates = &osrmv1alpha1.SpeedUpdatesSpec{
				Schedule: "0 2 * * *",
			}
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForInstanceDeployment(ctx, testInstance)

			By("Verifying CronJob is created")
			Eventually(func() error {
				cronJob := &batchv1.CronJob{}
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName("", osrmResource.CronJobSuffix),
					Namespace: testInstance.Namespace,
				}, cronJob)
			}, 30*time.Second).Should(Succeed())
		})

		It("should delete CronJob when SpeedUpdates is removed", func() {
			testInstance = generateOSRMInstance("speed-updates-removal")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForInstanceDeployment(ctx, testInstance)

			By("Adding SpeedUpdates")
			Expect(updateOSRMInstanceWithRetry(testInstance, func(v *osrmv1alpha1.OSRMInstance) {
				v.Spec.SpeedUpdates = &osrmv1alpha1.SpeedUpdatesSpec{
					Schedule: "0 2 * * *",
				}
			})).To(Succeed())

			Eventually(func() error {
				cronJob := &batchv1.CronJob{}
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName("", osrmResource.CronJobSuffix),
					Namespace: testInstance.Namespace,
				}, cronJob)
			}, 30*time.Second).Should(Succeed())

			By("Removing SpeedUpdates")
			Expect(updateOSRMInstanceWithRetry(testInstance, func(v *osrmv1alpha1.OSRMInstance) {
				v.Spec.SpeedUpdates = nil
			})).To(Succeed())

			By("Verifying CronJob is deleted")
			Eventually(func() bool {
				cronJob := &batchv1.CronJob{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName("", osrmResource.CronJobSuffix),
					Namespace: testInstance.Namespace,
				}, cronJob)
				return errors.IsNotFound(err)
			}, 30*time.Second).Should(BeTrue())
		})
	})

	Context("Pause Reconciliation Integration Tests", func() {
		var testInstance *osrmv1alpha1.OSRMInstance

		BeforeEach(func() {
			testInstance = generateOSRMInstance("pause-reconcile")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForInstanceDeployment(ctx, testInstance)
		})

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())
				testInstance = nil
			}
		})

		It("should skip reconciliation when pause annotation is set", func() {
			minReplicas := int32(2)
			originalMinReplicas := *testInstance.Spec.MinReplicas

			Expect(updateOSRMInstanceWithRetry(testInstance, func(v *osrmv1alpha1.OSRMInstance) {
				v.SetAnnotations(map[string]string{"osrm.itayankri/operator.paused": "true"})
				v.Spec.MinReplicas = &minReplicas
			})).To(Succeed())

			// Verify HPA still has old minReplicas (change not applied while paused)
			Eventually(func() int32 {
				hpa := &autoscalingv1.HorizontalPodAutoscaler{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName("", osrmResource.HorizontalPodAutoscalerSuffix),
					Namespace: testInstance.Namespace,
				}, hpa)
				if err != nil {
					return 0
				}
				return *hpa.Spec.MinReplicas
			}, MapBuildingTimeout).Should(Equal(originalMinReplicas))

			By("Resuming reconciliation")
			Expect(updateOSRMInstanceWithRetry(testInstance, func(v *osrmv1alpha1.OSRMInstance) {
				v.SetAnnotations(map[string]string{"osrm.itayankri/operator.paused": "false"})
			})).To(Succeed())

			Eventually(func() int32 {
				hpa := &autoscalingv1.HorizontalPodAutoscaler{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName("", osrmResource.HorizontalPodAutoscalerSuffix),
					Namespace: testInstance.Namespace,
				}, hpa)
				if err != nil {
					return 0
				}
				return *hpa.Spec.MinReplicas
			}, 10*time.Second).Should(Equal(minReplicas))
		})
	})

	Context("Reconciliation Status Integration Tests", func() {
		var testInstance *osrmv1alpha1.OSRMInstance

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())
				testInstance = nil
			}
		})

		It("should set ReconciliationSuccess to True when instance is healthy", func() {
			testInstance = generateOSRMInstance("reconcile-success")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())

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

		It("should set ReconciliationSuccess to False when spec is invalid", func() {
			testInstance = generateOSRMInstance("reconcile-failure")
			ptr := new(int32)
			*ptr = -1
			testInstance.Spec.MinReplicas = ptr
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())

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
			}, 180*time.Second).Should(Equal(metav1.ConditionFalse))
		})
	})

	Context("Resource Requirements Integration Tests", func() {
		var testInstance *osrmv1alpha1.OSRMInstance

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())
				testInstance = nil
			}
		})

		It("should apply resource requirements from spec to deployment", func() {
			testInstance = generateOSRMInstance("resource-requirements")
			expectedResources := corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			}
			testInstance.Spec.Resources = &expectedResources
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForInstanceDeployment(ctx, testInstance)

			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      testInstance.ChildResourceName("", osrmResource.DeploymentSuffix),
				Namespace: testInstance.Namespace,
			}, deployment)).To(Succeed())

			actualResources := deployment.Spec.Template.Spec.Containers[0].Resources
			Expect(actualResources).To(Equal(expectedResources))
		})

		It("should update deployment resource requirements when spec changes", func() {
			testInstance = generateOSRMInstance("resource-requirements-update")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForInstanceDeployment(ctx, testInstance)

			expectedRequirements := &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("50Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("50Mi"),
				},
			}

			Expect(updateOSRMInstanceWithRetry(testInstance, func(v *osrmv1alpha1.OSRMInstance) {
				v.Spec.Resources = expectedRequirements
			})).To(Succeed())

			Eventually(func() corev1.ResourceList {
				deployment := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName("", osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, deployment)).To(Succeed())
				return deployment.Spec.Template.Spec.Containers[0].Resources.Requests
			}, 30*time.Second).Should(HaveKeyWithValue(corev1.ResourceCPU, expectedRequirements.Requests[corev1.ResourceCPU]))
		})
	})

	Context("Resource Recreation Integration Tests", func() {
		var testInstance *osrmv1alpha1.OSRMInstance

		BeforeEach(func() {
			testInstance = generateOSRMInstance("recreate-children")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForInstanceDeployment(ctx, testInstance)
		})

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())
				testInstance = nil
			}
		})

		It("should recreate child resources after deletion", func() {
			oldDeployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      testInstance.ChildResourceName("", osrmResource.DeploymentSuffix),
				Namespace: testInstance.Namespace,
			}, oldDeployment)).To(Succeed())

			oldService := &corev1.Service{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      testInstance.ChildResourceName("", osrmResource.ServiceSuffix),
				Namespace: testInstance.Namespace,
			}, oldService)).To(Succeed())

			Expect(k8sClient.Delete(ctx, oldDeployment)).To(Succeed())
			Expect(k8sClient.Delete(ctx, oldService)).To(Succeed())

			Eventually(func() bool {
				newDeployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName("", osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, newDeployment)
				return err == nil && string(newDeployment.UID) != string(oldDeployment.UID)
			}, 30*time.Second).Should(BeTrue())

			Eventually(func() bool {
				newService := &corev1.Service{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName("", osrmResource.ServiceSuffix),
					Namespace: testInstance.Namespace,
				}, newService)
				return err == nil && string(newService.UID) != string(oldService.UID)
			}, 30*time.Second).Should(BeTrue())
		})
	})

	Context("Garbage Collection Integration Tests", func() {
		var testInstance *osrmv1alpha1.OSRMInstance

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())
				testInstance = nil
			}
		})

		It("should clean up old map generation PVC after update completes", func() {
			testInstance = generateOSRMInstance("gc-map-generations")

			By("Creating initial instance")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForInstanceDeployment(ctx, testInstance)

			By("Triggering map generation update")
			Expect(updateOSRMInstanceWithRetry(testInstance, func(v *osrmv1alpha1.OSRMInstance) {
				v.Spec.PBFURL = "https://download.geofabrik.de/australia-oceania/marshall-islands-220101.osm.pbf"
			})).To(Succeed())

			By("Waiting for redeployment to complete")
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

			By("Verifying generation-1 PVC is cleaned up")
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

// Helper functions for OSRMInstance integration tests

func generateOSRMInstance(name string) *osrmv1alpha1.OSRMInstance {
	stor := resource.MustParse("10Mi")
	accessMode := corev1.ReadWriteOnce
	minReplicas := int32(1)
	maxReplicas := int32(3)

	serialNumber := atomic.AddInt64(&instanceIntegrationTestCounter, 1)
	uniqueName := fmt.Sprintf("%s-%d", name, serialNumber)

	return &osrmv1alpha1.OSRMInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uniqueName,
			Namespace: "default",
		},
		Spec: osrmv1alpha1.OSRMInstanceSpec{
			PBFURL:      "https://download.geofabrik.de/australia-oceania/marshall-islands-210101.osm.pbf",
			OSRMProfile: "car",
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

func waitForInstanceDeployment(ctx context.Context, instance *osrmv1alpha1.OSRMInstance) {
	EventuallyWithOffset(1, func() string {
		got := osrmv1alpha1.OSRMInstance{}
		if err := k8sClient.Get(
			ctx,
			types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace},
			&got,
		); err != nil {
			return fmt.Sprintf("%v+", err)
		}

		for _, condition := range got.Status.Conditions {
			if condition.Type == status.ConditionAvailable && condition.Status == metav1.ConditionTrue {
				return "ready"
			}
		}

		return "not ready"
	}, MapBuildingTimeout, 1*time.Second).Should(Equal("ready"))
}
