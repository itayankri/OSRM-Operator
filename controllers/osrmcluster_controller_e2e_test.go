package controllers_test

import (
	"fmt"
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

var e2eTestCounter int64

var _ = Describe("OSRMClusterController End-to-End Tests", func() {
	Context("Full Workflow Tests", func() {
		var testInstance *osrmv1alpha1.OSRMCluster

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())

				// Wait for cleanup to complete
				Eventually(func() bool {
					cluster := &osrmv1alpha1.OSRMCluster{}
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      testInstance.Name,
						Namespace: testInstance.Namespace,
					}, cluster)
					return errors.IsNotFound(err)
				}, ClusterDeletionTimeout).Should(BeTrue())

				testInstance = nil
			}
		})

		It("should handle complete cluster lifecycle: create → add profiles → update PBF → remove profiles → delete", func() {
			testInstance = generateE2EOSRMCluster("full-lifecycle-test")

			By("Step 1: Creating initial cluster")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForDeployment(ctx, testInstance, k8sClient)

			By("Step 1: Verifying initial state")
			Eventually(func() osrmv1alpha1.Phase {
				cluster := &osrmv1alpha1.OSRMCluster{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.Name,
					Namespace: testInstance.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, MapBuildingTimeout).Should(Equal(osrmv1alpha1.PhaseWorkersDeployed))

			By("Step 2: Adding new profile")
			osrmProfile := "foot"
			internalEndpoint := "walking"
			minReplicas := int32(1)
			maxReplicas := int32(2)
			newProfile := &osrmv1alpha1.ProfileSpec{
				Name:             "walking",
				EndpointName:     "walking-endpoint",
				InternalEndpoint: &internalEndpoint,
				OSRMProfile:      &osrmProfile,
				MinReplicas:      &minReplicas,
				MaxReplicas:      &maxReplicas,
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
			}

			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.Profiles = append(v.Spec.Profiles, newProfile)
			})).To(Succeed())

			By("Step 2: Verifying new profile resources are created")
			Eventually(func() error {
				deployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(newProfile.Name, osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, deployment)
				if err != nil {
					return err
				}

				service := &corev1.Service{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(newProfile.Name, osrmResource.ServiceSuffix),
					Namespace: testInstance.Namespace,
				}, service)
				return err
			}, MapBuildingTimeout).Should(Succeed())

			By("Step 3: Updating PBF URL")
			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.PBFURL = "https://download.geofabrik.de/europe/andorra-latest.osm.pbf"
			})).To(Succeed())

			By("Step 3: Verifying phase transition to UpdatingMap")
			Eventually(func() osrmv1alpha1.Phase {
				cluster := &osrmv1alpha1.OSRMCluster{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.Name,
					Namespace: testInstance.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, 30*time.Second).Should(Equal(osrmv1alpha1.PhaseUpdatingMap))

			By("Step 3: Verifying map update completes")
			Eventually(func() osrmv1alpha1.Phase {
				cluster := &osrmv1alpha1.OSRMCluster{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.Name,
					Namespace: testInstance.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, MapBuildingTimeout).Should(Or(
				Equal(osrmv1alpha1.PhaseWorkersDeployed),
				Equal(osrmv1alpha1.PhaseWorkersRedeployed),
			))

			By("Step 4: Removing one profile")
			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.Profiles = []*osrmv1alpha1.ProfileSpec{v.Spec.Profiles[0]} // Keep only first profile
			})).To(Succeed())

			By("Step 4: Verifying removed profile resources are cleaned up")
			Eventually(func() bool {
				deployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(newProfile.Name, osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, deployment)
				return errors.IsNotFound(err)
			}, 30*time.Second).Should(BeTrue())

			By("Step 4: Verifying remaining profile is unaffected")
			deployment := &appsv1.Deployment{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      testInstance.ChildResourceName(testInstance.Spec.Profiles[0].Name, osrmResource.DeploymentSuffix),
				Namespace: testInstance.Namespace,
			}, deployment)
			Expect(err).NotTo(HaveOccurred())

			By("Step 5: Removing all profiles")
			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.Profiles = []*osrmv1alpha1.ProfileSpec{}
			})).To(Succeed())

			By("Step 5: Verifying all profile resources are cleaned up")
			Eventually(func() bool {
				// Check all profile resources are deleted
				deployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName("car", osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, deployment)
				if !errors.IsNotFound(err) {
					return false
				}

				// Check gateway resources are cleaned up
				gatewayDeployment := &appsv1.Deployment{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(osrmResource.GatewaySuffix, osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, gatewayDeployment)
				if !errors.IsNotFound(err) {
					return false
				}

				return true
			}, 30*time.Second).Should(BeTrue())

			By("Step 5: Verifying cluster phase is Empty")
			Eventually(func() osrmv1alpha1.Phase {
				cluster := &osrmv1alpha1.OSRMCluster{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.Name,
					Namespace: testInstance.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, 30*time.Second).Should(Equal(osrmv1alpha1.PhaseEmpty))
		})

		It("should handle concurrent operations: simultaneous profile addition and PBF update", func() {
			testInstance = generateE2EOSRMCluster("concurrent-ops-test")

			By("Creating initial cluster")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForDeployment(ctx, testInstance, k8sClient)

			By("Making concurrent changes: adding profile and updating PBF")
			osrmProfile := "foot"
			internalEndpoint := "walking"
			minReplicas := int32(1)
			maxReplicas := int32(2)
			newProfile := &osrmv1alpha1.ProfileSpec{
				Name:             "walking",
				EndpointName:     "walking-endpoint",
				InternalEndpoint: &internalEndpoint,
				OSRMProfile:      &osrmProfile,
				MinReplicas:      &minReplicas,
				MaxReplicas:      &maxReplicas,
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
			}

			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.Profiles = append(v.Spec.Profiles, newProfile)
				v.Spec.PBFURL = "https://download.geofabrik.de/europe/andorra-latest.osm.pbf"
			})).To(Succeed())

			By("Verifying system reaches stable state")
			Eventually(func() osrmv1alpha1.Phase {
				cluster := &osrmv1alpha1.OSRMCluster{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.Name,
					Namespace: testInstance.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, MapBuildingTimeout).Should(Or(
				Equal(osrmv1alpha1.PhaseWorkersDeployed),
				Equal(osrmv1alpha1.PhaseWorkersRedeployed),
			))

			By("Verifying both profiles have resources")
			for _, profile := range testInstance.Spec.Profiles {
				Eventually(func() error {
					deployment := &appsv1.Deployment{}
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      testInstance.ChildResourceName(profile.Name, osrmResource.DeploymentSuffix),
						Namespace: testInstance.Namespace,
					}, deployment)
					if err != nil {
						return err
					}

					service := &corev1.Service{}
					err = k8sClient.Get(ctx, types.NamespacedName{
						Name:      testInstance.ChildResourceName(profile.Name, osrmResource.ServiceSuffix),
						Namespace: testInstance.Namespace,
					}, service)
					return err
				}, MapBuildingTimeout).Should(Succeed())
			}

			By("Verifying reconciliation eventually succeeds")
			Eventually(func() metav1.ConditionStatus {
				cluster := &osrmv1alpha1.OSRMCluster{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.Name,
					Namespace: testInstance.Namespace,
				}, cluster)
				if err != nil {
					return metav1.ConditionUnknown
				}

				for _, condition := range cluster.Status.Conditions {
					if condition.Type == status.ConditionReconciliationSuccess {
						return condition.Status
					}
				}
				return metav1.ConditionUnknown
			}, MapBuildingTimeout).Should(Equal(metav1.ConditionTrue))
		})
	})

	Context("Performance Tests", func() {
		var testInstance *osrmv1alpha1.OSRMCluster

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())
				testInstance = nil
			}
		})

		It("should handle cluster with multiple profiles efficiently", func() {
			testInstance = generateLargeOSRMCluster("performance-test", 5)

			By("Creating cluster with 5 profiles")
			startTime := time.Now()
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())

			By("Verifying all profiles are deployed within reasonable time")
			Eventually(func() bool {
				cluster := &osrmv1alpha1.OSRMCluster{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.Name,
					Namespace: testInstance.Namespace,
				}, cluster)
				if err != nil {
					return false
				}

				return cluster.Status.Phase == osrmv1alpha1.PhaseWorkersDeployed
			}, MapBuildingTimeout).Should(BeTrue())

			deploymentTime := time.Since(startTime)
			By(fmt.Sprintf("Deployment completed in %v", deploymentTime))

			By("Verifying all profile resources exist")
			for _, profile := range testInstance.Spec.Profiles {
				deployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(profile.Name, osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, deployment)
				Expect(err).NotTo(HaveOccurred())
			}

			By("Testing rapid profile removal")
			startTime = time.Now()
			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				// Remove all but first profile
				v.Spec.Profiles = []*osrmv1alpha1.ProfileSpec{v.Spec.Profiles[0]}
			})).To(Succeed())

			By("Verifying cleanup completes efficiently")
			Eventually(func() bool {
				// Check that removed profiles are cleaned up
				for i := 1; i < 5; i++ {
					profileName := fmt.Sprintf("profile-%d", i)
					deployment := &appsv1.Deployment{}
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      testInstance.ChildResourceName(profileName, osrmResource.DeploymentSuffix),
						Namespace: testInstance.Namespace,
					}, deployment)
					if !errors.IsNotFound(err) {
						return false
					}
				}
				return true
			}, 60*time.Second).Should(BeTrue())

			cleanupTime := time.Since(startTime)
			By(fmt.Sprintf("Cleanup completed in %v", cleanupTime))
		})
	})

	Context("Failure Recovery Tests", func() {
		var testInstance *osrmv1alpha1.OSRMCluster

		BeforeEach(func() {
			testInstance = generateE2EOSRMCluster("failure-recovery-test")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForDeployment(ctx, testInstance, k8sClient)
		})

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())
				testInstance = nil
			}
		})

		It("should recreate manually deleted resources", func() {
			By("Manually deleting profile deployment")
			deployment := &appsv1.Deployment{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      testInstance.ChildResourceName(testInstance.Spec.Profiles[0].Name, osrmResource.DeploymentSuffix),
				Namespace: testInstance.Namespace,
			}, deployment)
			Expect(err).NotTo(HaveOccurred())

			originalUID := deployment.UID
			Expect(k8sClient.Delete(ctx, deployment)).To(Succeed())

			By("Verifying deployment is recreated")
			Eventually(func() bool {
				newDeployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(testInstance.Spec.Profiles[0].Name, osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, newDeployment)
				if err != nil {
					return false
				}

				return newDeployment.UID != originalUID
			}, 30*time.Second).Should(BeTrue())
		})
	})

	Context("Garbage Collection End-to-End Tests", func() {
		var testInstance *osrmv1alpha1.OSRMCluster

		BeforeEach(func() {
			testInstance = generateE2EOSRMCluster("gc-e2e-test")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForDeployment(ctx, testInstance, k8sClient)
		})

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())
				testInstance = nil
			}
		})

		It("should not have duplicate deletion errors during PBF updates", func() {
			By("Updating PBF URL to trigger map update")
			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.PBFURL = "https://download.geofabrik.de/europe/andorra-latest.osm.pbf"
			})).To(Succeed())

			By("Verifying system progresses through phases without errors")
			Eventually(func() osrmv1alpha1.Phase {
				cluster := &osrmv1alpha1.OSRMCluster{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.Name,
					Namespace: testInstance.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, MapBuildingTimeout).Should(Or(
				Equal(osrmv1alpha1.PhaseWorkersDeployed),
				Equal(osrmv1alpha1.PhaseWorkersRedeployed),
			))

			By("Verifying reconciliation succeeds without deletion errors")
			Eventually(func() metav1.ConditionStatus {
				cluster := &osrmv1alpha1.OSRMCluster{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.Name,
					Namespace: testInstance.Namespace,
				}, cluster)
				if err != nil {
					return metav1.ConditionUnknown
				}

				for _, condition := range cluster.Status.Conditions {
					if condition.Type == status.ConditionReconciliationSuccess {
						return condition.Status
					}
				}
				return metav1.ConditionUnknown
			}, 30*time.Second).Should(Equal(metav1.ConditionTrue))
		})

		It("should clean up resources for removed profiles without affecting others", func() {
			By("Adding additional profiles")
			profiles := []*osrmv1alpha1.ProfileSpec{
				testInstance.Spec.Profiles[0], // Keep original
				createTestProfile("walking", "walking-endpoint"),
				createTestProfile("cycling", "cycling-endpoint"),
			}

			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.Profiles = profiles
			})).To(Succeed())

			By("Waiting for all profiles to be deployed")
			Eventually(func() bool {
				for _, profile := range profiles {
					deployment := &appsv1.Deployment{}
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      testInstance.ChildResourceName(profile.Name, osrmResource.DeploymentSuffix),
						Namespace: testInstance.Namespace,
					}, deployment)
					if err != nil {
						return false
					}
				}
				return true
			}, MapBuildingTimeout).Should(BeTrue())

			By("Removing middle profile")
			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.Profiles = []*osrmv1alpha1.ProfileSpec{
					profiles[0], // car
					profiles[2], // cycling
				}
			})).To(Succeed())

			By("Verifying removed profile resources are cleaned up")
			Eventually(func() bool {
				resources := []string{
					testInstance.ChildResourceName("walking", osrmResource.DeploymentSuffix),
					testInstance.ChildResourceName("walking", osrmResource.ServiceSuffix),
					testInstance.ChildResourceName("walking", osrmResource.HorizontalPodAutoscalerSuffix),
					testInstance.ChildResourceName("walking", osrmResource.PodDisruptionBudgetSuffix),
				}

				for _, resourceName := range resources {
					deployment := &appsv1.Deployment{}
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      resourceName,
						Namespace: testInstance.Namespace,
					}, deployment)
					if !errors.IsNotFound(err) {
						return false
					}
				}
				return true
			}, 30*time.Second).Should(BeTrue())

			By("Verifying remaining profiles are unaffected")
			for _, profileName := range []string{"car", "cycling"} {
				deployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(profileName, osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, deployment)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})
})

// Helper functions for e2e tests
func generateE2EOSRMCluster(name string) *osrmv1alpha1.OSRMCluster {
	storage := resource.MustParse("10Mi")
	accessMode := corev1.ReadWriteOnce
	minReplicas := int32(1)
	maxReplicas := int32(3)

	serialNumber := atomic.AddInt64(&e2eTestCounter, 1)
	uniqueName := fmt.Sprintf("%s-%d", name, serialNumber)

	return &osrmv1alpha1.OSRMCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uniqueName,
			Namespace: "default",
		},
		Spec: osrmv1alpha1.OSRMClusterSpec{
			PBFURL: "https://download.geofabrik.de/australia-oceania/marshall-islands-210101.osm.pbf",
			Persistence: osrmv1alpha1.PersistenceSpec{
				StorageClassName: "standard",
				Storage:          &storage,
				AccessMode:       &accessMode,
			},
			Profiles: []*osrmv1alpha1.ProfileSpec{
				{
					Name:         "car",
					EndpointName: "driving",
					MinReplicas:  &minReplicas,
					MaxReplicas:  &maxReplicas,
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
			},
			Service: osrmv1alpha1.ServiceSpec{
				ExposingServices: []string{"route"},
			},
		},
	}
}

func generateLargeOSRMCluster(name string, profileCount int) *osrmv1alpha1.OSRMCluster {
	cluster := generateE2EOSRMCluster(name)
	cluster.Spec.Profiles = []*osrmv1alpha1.ProfileSpec{}

	for i := 0; i < profileCount; i++ {
		profile := createTestProfile(fmt.Sprintf("profile-%d", i), fmt.Sprintf("endpoint-%d", i))
		cluster.Spec.Profiles = append(cluster.Spec.Profiles, profile)
	}

	return cluster
}

func createTestProfile(name, endpoint string) *osrmv1alpha1.ProfileSpec {
	osrmProfile := "car"
	minReplicas := int32(1)
	maxReplicas := int32(3)

	return &osrmv1alpha1.ProfileSpec{
		Name:         name,
		EndpointName: endpoint,
		OSRMProfile:  &osrmProfile,
		MinReplicas:  &minReplicas,
		MaxReplicas:  &maxReplicas,
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
	}
}
