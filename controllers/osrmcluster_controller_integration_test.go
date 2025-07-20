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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var integrationTestCounter int64

const (
	ClusterDeletionTimeout = 5 * time.Second
	MapBuildingTimeout     = 2 * 60 * time.Second
)

var _ = Describe("OSRMClusterController Integration Tests", func() {
	Context("Basic Lifecycle Tests", func() {
		var testInstance *osrmv1alpha1.OSRMCluster

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())
				testInstance = nil
			}
		})

		It("should create cluster with single profile and progress through phases", func() {
			testInstance = generateOSRMCluster("single-profile-lifecycle")

			By("Creating the OSRMCluster")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())

			By("Verifying the cluster progresses through phases")
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
				Equal(osrmv1alpha1.PhaseBuildingMap),
				Equal(osrmv1alpha1.PhaseDeployingWorkers),
				Equal(osrmv1alpha1.PhaseWorkersDeployed),
			))

			By("Verifying all expected resources are created")
			Eventually(func() error {
				// Check profile deployment
				deployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(testInstance.Spec.Profiles[0].Name, osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, deployment)
				if err != nil {
					return err
				}

				// Check profile service
				service := &corev1.Service{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(testInstance.Spec.Profiles[0].Name, osrmResource.ServiceSuffix),
					Namespace: testInstance.Namespace,
				}, service)
				if err != nil {
					return err
				}

				// Check gateway deployment
				gatewayDeployment := &appsv1.Deployment{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(osrmResource.GatewaySuffix, osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, gatewayDeployment)
				if err != nil {
					return err
				}

				return nil
			}, MapBuildingTimeout).Should(Succeed())
		})

		It("should create cluster with multiple profiles", func() {
			testInstance = generateMultiProfileOSRMCluster("multi-profile-lifecycle")

			By("Creating the OSRMCluster with multiple profiles")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())

			By("Verifying resources are created for all profiles")
			for _, profile := range testInstance.Spec.Profiles {
				Eventually(func() error {
					// Check profile deployment
					deployment := &appsv1.Deployment{}
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      testInstance.ChildResourceName(profile.Name, osrmResource.DeploymentSuffix),
						Namespace: testInstance.Namespace,
					}, deployment)
					if err != nil {
						return err
					}

					// Check profile service
					service := &corev1.Service{}
					err = k8sClient.Get(ctx, types.NamespacedName{
						Name:      testInstance.ChildResourceName(profile.Name, osrmResource.ServiceSuffix),
						Namespace: testInstance.Namespace,
					}, service)
					if err != nil {
						return err
					}

					// Check HPA
					hpa := &autoscalingv1.HorizontalPodAutoscaler{}
					err = k8sClient.Get(ctx, types.NamespacedName{
						Name:      testInstance.ChildResourceName(profile.Name, osrmResource.HorizontalPodAutoscalerSuffix),
						Namespace: testInstance.Namespace,
					}, hpa)
					if err != nil {
						return err
					}

					// Check PDB
					pdb := &policyv1.PodDisruptionBudget{}
					err = k8sClient.Get(ctx, types.NamespacedName{
						Name:      testInstance.ChildResourceName(profile.Name, osrmResource.PodDisruptionBudgetSuffix),
						Namespace: testInstance.Namespace,
					}, pdb)
					if err != nil {
						return err
					}

					return nil
				}, MapBuildingTimeout).Should(Succeed())
			}

			By("Verifying gateway resources are created")
			Eventually(func() error {
				// Check gateway deployment
				gatewayDeployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(osrmResource.GatewaySuffix, osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, gatewayDeployment)
				if err != nil {
					return err
				}

				// Check gateway service
				gatewayService := &corev1.Service{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(osrmResource.GatewaySuffix, osrmResource.ServiceSuffix),
					Namespace: testInstance.Namespace,
				}, gatewayService)
				if err != nil {
					return err
				}

				// Check gateway config map
				gatewayConfigMap := &corev1.ConfigMap{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(osrmResource.GatewaySuffix, osrmResource.ConfigMapSuffix),
					Namespace: testInstance.Namespace,
				}, gatewayConfigMap)
				if err != nil {
					return err
				}

				return nil
			}, MapBuildingTimeout).Should(Succeed())
		})
	})

	Context("Map Update Tests", func() {
		var testInstance *osrmv1alpha1.OSRMCluster

		BeforeEach(func() {
			testInstance = generateOSRMCluster("map-update-test")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForDeployment(ctx, testInstance, k8sClient)
		})

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())
				testInstance = nil
			}
		})

		It("should transition to UpdatingMap phase when PBF URL changes", func() {
			By("Updating the PBF URL")
			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.PBFURL = "https://download.geofabrik.de/europe/andorra-latest.osm.pbf"
			})).To(Succeed())

			By("Verifying phase transition to UpdatingMap")
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

			By("Verifying new PVCs and Jobs are created with incremented map generation")
			Eventually(func() bool {
				// Check for generation 2 PVC
				pvc := &corev1.PersistentVolumeClaim{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(testInstance.Spec.Profiles[0].Name, "2"),
					Namespace: testInstance.Namespace,
				}, pvc)
				if err != nil {
					return false
				}

				// Check for generation 2 Job
				job := &batchv1.Job{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(testInstance.Spec.Profiles[0].Name, "2"),
					Namespace: testInstance.Namespace,
				}, job)
				if err != nil {
					return false
				}

				return true
			}, 60*time.Second).Should(BeTrue())
		})

		It("should handle multiple successive PBF URL changes", func() {
			By("Making first PBF URL change")
			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.PBFURL = "https://download.geofabrik.de/europe/andorra-latest.osm.pbf"
			})).To(Succeed())

			By("Waiting for first update to start")
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

			By("Making second PBF URL change")
			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.PBFURL = "https://download.geofabrik.de/europe/monaco-latest.osm.pbf"
			})).To(Succeed())

			By("Verifying system handles multiple changes gracefully")
			Consistently(func() osrmv1alpha1.Phase {
				cluster := &osrmv1alpha1.OSRMCluster{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.Name,
					Namespace: testInstance.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, 15*time.Second).Should(Or(
				Equal(osrmv1alpha1.PhaseUpdatingMap),
				Equal(osrmv1alpha1.PhaseRedepoloyingWorkers),
				Equal(osrmv1alpha1.PhaseWorkersRedeployed),
			))
		})
	})

	Context("Profile Management Tests", func() {
		var testInstance *osrmv1alpha1.OSRMCluster

		BeforeEach(func() {
			testInstance = generateOSRMCluster("profile-management-test")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForDeployment(ctx, testInstance, k8sClient)
		})

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())
				testInstance = nil
			}
		})

		It("should add new profile to existing cluster", func() {
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
			}

			By("Adding new profile to cluster")
			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.Profiles = append(v.Spec.Profiles, newProfile)
			})).To(Succeed())

			By("Verifying new profile resources are created")
			Eventually(func() error {
				// Check profile deployment
				deployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(newProfile.Name, osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, deployment)
				if err != nil {
					return err
				}

				// Check profile service
				service := &corev1.Service{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(newProfile.Name, osrmResource.ServiceSuffix),
					Namespace: testInstance.Namespace,
				}, service)
				if err != nil {
					return err
				}

				return nil
			}, MapBuildingTimeout).Should(Succeed())

			By("Verifying existing profile is unaffected")
			deployment := &appsv1.Deployment{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      testInstance.ChildResourceName(testInstance.Spec.Profiles[0].Name, osrmResource.DeploymentSuffix),
				Namespace: testInstance.Namespace,
			}, deployment)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should remove profile from cluster and clean up resources", func() {
			originalProfile := testInstance.Spec.Profiles[0]

			By("Removing profile from cluster")
			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.Profiles = []*osrmv1alpha1.ProfileSpec{}
			})).To(Succeed())

			By("Verifying profile resources are cleaned up")
			Eventually(func() bool {
				// Check profile deployment is deleted
				deployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(originalProfile.Name, osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, deployment)
				if !errors.IsNotFound(err) {
					return false
				}

				// Check profile service is deleted
				service := &corev1.Service{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(originalProfile.Name, osrmResource.ServiceSuffix),
					Namespace: testInstance.Namespace,
				}, service)
				if !errors.IsNotFound(err) {
					return false
				}

				// Check HPA is deleted
				hpa := &autoscalingv1.HorizontalPodAutoscaler{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(originalProfile.Name, osrmResource.HorizontalPodAutoscalerSuffix),
					Namespace: testInstance.Namespace,
				}, hpa)
				if !errors.IsNotFound(err) {
					return false
				}

				// Check PDB is deleted
				pdb := &policyv1.PodDisruptionBudget{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(originalProfile.Name, osrmResource.PodDisruptionBudgetSuffix),
					Namespace: testInstance.Namespace,
				}, pdb)
				if !errors.IsNotFound(err) {
					return false
				}

				return true
			}, 30*time.Second).Should(BeTrue())

			By("Verifying gateway resources are cleaned up when no profiles exist")
			Eventually(func() bool {
				// Check gateway deployment is deleted
				gatewayDeployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(osrmResource.GatewaySuffix, osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, gatewayDeployment)
				if !errors.IsNotFound(err) {
					return false
				}

				// Check gateway service is deleted
				gatewayService := &corev1.Service{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(osrmResource.GatewaySuffix, osrmResource.ServiceSuffix),
					Namespace: testInstance.Namespace,
				}, gatewayService)
				if !errors.IsNotFound(err) {
					return false
				}

				return true
			}, 30*time.Second).Should(BeTrue())
		})

		It("should handle speed updates configuration changes", func() {
			By("Adding speed updates to existing profile")
			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.Profiles[0].SpeedUpdates = &osrmv1alpha1.SpeedUpdatesSpec{
					Schedule: "0 2 * * *",
				}
			})).To(Succeed())

			By("Verifying CronJob is created")
			Eventually(func() error {
				cronJob := &batchv1.CronJob{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(testInstance.Spec.Profiles[0].Name, osrmResource.CronJobSuffix),
					Namespace: testInstance.Namespace,
				}, cronJob)
				return err
			}, 30*time.Second).Should(Succeed())

			By("Removing speed updates from profile")
			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.Profiles[0].SpeedUpdates = nil
			})).To(Succeed())

			By("Verifying CronJob is deleted")
			Eventually(func() bool {
				cronJob := &batchv1.CronJob{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(testInstance.Spec.Profiles[0].Name, osrmResource.CronJobSuffix),
					Namespace: testInstance.Namespace,
				}, cronJob)
				return errors.IsNotFound(err)
			}, 30*time.Second).Should(BeTrue())
		})
	})

	Context("Error Handling Tests", func() {
		var testInstance *osrmv1alpha1.OSRMCluster

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())
				testInstance = nil
			}
		})

		It("should handle invalid resource requirements", func() {
			testInstance = generateOSRMCluster("error-handling-test")

			By("Setting invalid resource requirements")
			ptr := new(int32)
			*ptr = -1
			testInstance.Spec.Profiles[0].MinReplicas = ptr

			By("Creating the cluster")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())

			By("Verifying reconciliation fails")
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
			}, 30*time.Second).Should(Equal(metav1.ConditionFalse))

			By("Fixing the resource requirements")
			*ptr = 2
			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.Profiles[0].MinReplicas = ptr
			})).To(Succeed())

			By("Verifying reconciliation succeeds after fix")
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
			}, 60*time.Second).Should(Equal(metav1.ConditionTrue))
		})
	})

	Context("Resource Requirements Integration Tests", func() {
		var testInstance *osrmv1alpha1.OSRMCluster

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())
				testInstance = nil
			}
		})

		It("should use resource requirements from profile spec when provided", func() {
			testInstance = generateOSRMCluster("resource-requirements-config")
			expectedResources := corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			}
			testInstance.Spec.Profiles[0].Resources = &expectedResources
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForDeployment(ctx, testInstance, k8sClient)

			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      testInstance.ChildResourceName(testInstance.Spec.Profiles[0].Name, osrmResource.DeploymentSuffix),
				Namespace: testInstance.Namespace,
			}, deployment)).To(Succeed())

			actualResources := deployment.Spec.Template.Spec.Containers[0].Resources
			Expect(actualResources).To(Equal(expectedResources))
		})

		It("should update deployment CPU and memory requests and limits", func() {
			testInstance = generateOSRMCluster("custom-resource-updates")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForDeployment(ctx, testInstance, k8sClient)

			expectedRequirements := &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("200Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("200Mi"),
				},
			}

			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.Profiles[0].Resources = expectedRequirements
			})).To(Succeed())

			Eventually(func() corev1.ResourceList {
				deployment := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(testInstance.Spec.Profiles[0].Name, osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, deployment)).To(Succeed())
				return deployment.Spec.Template.Spec.Containers[0].Resources.Requests
			}, 30*time.Second).Should(HaveKeyWithValue(corev1.ResourceCPU, expectedRequirements.Requests[corev1.ResourceCPU]))
		})
	})

	Context("ConfigMap Integration Tests", func() {
		var testInstance *osrmv1alpha1.OSRMCluster

		BeforeEach(func() {
			testInstance = generateOSRMCluster("configmap-integration-test")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForDeployment(ctx, testInstance, k8sClient)
		})

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())
				testInstance = nil
			}
		})

		It("should rollout gateway deployment after adding a new profile", func() {
			// Get initial gateway config version
			gatewayDeployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      testInstance.ChildResourceName("", osrmResource.DeploymentSuffix),
				Namespace: testInstance.Namespace,
			}, gatewayDeployment)).To(Succeed())
			gatewayConfigVersionAnnotation := gatewayDeployment.Spec.Template.ObjectMeta.Annotations[osrmResource.GatewayConfigVersion]

			osrmProfile := "foot"
			internalEndpoint := "walking"
			minReplicas := int32(1)
			maxReplicas := int32(2)
			newProfile := &osrmv1alpha1.ProfileSpec{
				Name:             "new-profile",
				EndpointName:     "custom-endpoint",
				InternalEndpoint: &internalEndpoint,
				OSRMProfile:      &osrmProfile,
				MinReplicas:      &minReplicas,
				MaxReplicas:      &maxReplicas,
			}

			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.Profiles = append(v.Spec.Profiles, newProfile)
			})).To(Succeed())

			Eventually(func() string {
				gatewayDeployment := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName("", osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, gatewayDeployment)).To(Succeed())
				return gatewayDeployment.Spec.Template.ObjectMeta.Annotations[osrmResource.GatewayConfigVersion]
			}, 180*time.Second).ShouldNot(Equal(gatewayConfigVersionAnnotation))
		})

		It("should rollout gateway deployment after modifying ExposingServices", func() {
			// Get initial gateway config version
			gatewayDeployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      testInstance.ChildResourceName("", osrmResource.DeploymentSuffix),
				Namespace: testInstance.Namespace,
			}, gatewayDeployment)).To(Succeed())
			gatewayConfigVersionAnnotation := gatewayDeployment.Spec.Template.ObjectMeta.Annotations[osrmResource.GatewayConfigVersion]

			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.Service.ExposingServices = append(v.Spec.Service.ExposingServices, "table")
			})).To(Succeed())

			Eventually(func() string {
				gatewayDeployment := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName("", osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, gatewayDeployment)).To(Succeed())
				return gatewayDeployment.Spec.Template.ObjectMeta.Annotations[osrmResource.GatewayConfigVersion]
			}, 180*time.Second).ShouldNot(Equal(gatewayConfigVersionAnnotation))
		})

		It("should rollout gateway deployment after editing a profile's EndpointName", func() {
			// Get initial gateway config version
			gatewayDeployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      testInstance.ChildResourceName("", osrmResource.DeploymentSuffix),
				Namespace: testInstance.Namespace,
			}, gatewayDeployment)).To(Succeed())
			gatewayConfigVersionAnnotation := gatewayDeployment.Spec.Template.ObjectMeta.Annotations[osrmResource.GatewayConfigVersion]

			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.Profiles[0].EndpointName = "ankri"
			})).To(Succeed())

			Eventually(func() string {
				gatewayDeployment := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName("", osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, gatewayDeployment)).To(Succeed())
				return gatewayDeployment.Spec.Template.ObjectMeta.Annotations[osrmResource.GatewayConfigVersion]
			}, 180*time.Second).ShouldNot(Equal(gatewayConfigVersionAnnotation))
		})
	})

	Context("Resource Recreation Integration Tests", func() {
		var testInstance *osrmv1alpha1.OSRMCluster

		BeforeEach(func() {
			testInstance = generateOSRMCluster("recreate-children")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForDeployment(ctx, testInstance, k8sClient)
		})

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())
				testInstance = nil
			}
		})

		It("should recreate child resources after deletion", func() {
			// Get original resources
			oldService := &corev1.Service{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      testInstance.ChildResourceName(testInstance.Spec.Profiles[0].Name, osrmResource.ServiceSuffix),
				Namespace: testInstance.Namespace,
			}, oldService)).To(Succeed())

			oldDeployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      testInstance.ChildResourceName(testInstance.Spec.Profiles[0].Name, osrmResource.DeploymentSuffix),
				Namespace: testInstance.Namespace,
			}, oldDeployment)).To(Succeed())

			oldHpa := &autoscalingv1.HorizontalPodAutoscaler{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      testInstance.ChildResourceName(testInstance.Spec.Profiles[0].Name, osrmResource.HorizontalPodAutoscalerSuffix),
				Namespace: testInstance.Namespace,
			}, oldHpa)).To(Succeed())

			// Delete resources
			Expect(k8sClient.Delete(ctx, oldService)).To(Succeed())
			Expect(k8sClient.Delete(ctx, oldHpa)).To(Succeed())
			Expect(k8sClient.Delete(ctx, oldDeployment)).To(Succeed())

			// Verify recreation
			Eventually(func() bool {
				newDeployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(testInstance.Spec.Profiles[0].Name, osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, newDeployment)
				return err == nil && string(newDeployment.UID) != string(oldDeployment.UID)
			}, 30*time.Second).Should(BeTrue())

			Eventually(func() bool {
				newService := &corev1.Service{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(testInstance.Spec.Profiles[0].Name, osrmResource.ServiceSuffix),
					Namespace: testInstance.Namespace,
				}, newService)
				return err == nil && string(newService.UID) != string(oldService.UID)
			}, 30*time.Second).Should(BeTrue())

			Eventually(func() bool {
				newHpa := &autoscalingv1.HorizontalPodAutoscaler{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(testInstance.Spec.Profiles[0].Name, osrmResource.HorizontalPodAutoscalerSuffix),
					Namespace: testInstance.Namespace,
				}, newHpa)
				return err == nil && string(newHpa.UID) != string(oldHpa.UID)
			}, 30*time.Second).Should(BeTrue())
		})
	})

	Context("Reconciliation Status Integration Tests", func() {
		var testInstance *osrmv1alpha1.OSRMCluster

		BeforeEach(func() {
			testInstance = generateOSRMCluster("reconcile-success-condition")
		})

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())
				testInstance = nil
			}
		})

		It("should keep ReconcileSuccess condition updated", func() {
			By("Setting to False when spec is not valid")
			ptr := new(int32)
			*ptr = -1
			// It is impossible to create a deployment with -1 replicas. Thus we expect reconciliation to fail.
			testInstance.Spec.Profiles[0].MinReplicas = ptr
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())

			Eventually(func() metav1.ConditionStatus {
				osrmCluster := &osrmv1alpha1.OSRMCluster{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.Name,
					Namespace: testInstance.Namespace,
				}, osrmCluster)
				if err != nil {
					return metav1.ConditionUnknown
				}

				for _, condition := range osrmCluster.Status.Conditions {
					if condition.Type == status.ConditionReconciliationSuccess {
						return condition.Status
					}
				}
				return metav1.ConditionUnknown
			}, 180*time.Second).Should(Equal(metav1.ConditionFalse))

			By("Setting to True when spec is valid")
			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				ptr := new(int32)
				*ptr = 2
				v.Spec.Profiles[0].MinReplicas = ptr
			})).To(Succeed())

			Eventually(func() metav1.ConditionStatus {
				osrmCluster := &osrmv1alpha1.OSRMCluster{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.Name,
					Namespace: testInstance.Namespace,
				}, osrmCluster)
				if err != nil {
					return metav1.ConditionUnknown
				}

				for _, condition := range osrmCluster.Status.Conditions {
					if condition.Type == status.ConditionReconciliationSuccess {
						return condition.Status
					}
				}
				return metav1.ConditionUnknown
			}, 60*time.Second).Should(Equal(metav1.ConditionTrue))
		})
	})

	Context("Pause Reconciliation Integration Tests", func() {
		var testInstance *osrmv1alpha1.OSRMCluster

		BeforeEach(func() {
			testInstance = generateOSRMCluster("pause-reconcile")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForDeployment(ctx, testInstance, k8sClient)
		})

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())
				testInstance = nil
			}
		})

		It("should skip OSRMCluster if pause reconciliation annotation is set to true", func() {
			minReplicas := int32(2)
			originalMinReplicas := *testInstance.Spec.Profiles[0].MinReplicas

			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.SetAnnotations(map[string]string{"osrm.itayankri/operator.paused": "true"})
				v.Spec.Profiles[0].MinReplicas = &minReplicas
			})).To(Succeed())

			Eventually(func() int32 {
				hpa := &autoscalingv1.HorizontalPodAutoscaler{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(testInstance.Spec.Profiles[0].Name, osrmResource.HorizontalPodAutoscalerSuffix),
					Namespace: testInstance.Namespace,
				}, hpa)
				if err != nil {
					return 0
				}
				return *hpa.Spec.MinReplicas
			}, MapBuildingTimeout).Should(Equal(originalMinReplicas))

			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.SetAnnotations(map[string]string{"osrm.itayankri/operator.paused": "false"})
			})).To(Succeed())

			Eventually(func() int32 {
				hpa := &autoscalingv1.HorizontalPodAutoscaler{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(testInstance.Spec.Profiles[0].Name, osrmResource.HorizontalPodAutoscalerSuffix),
					Namespace: testInstance.Namespace,
				}, hpa)
				if err != nil {
					return 0
				}
				return *hpa.Spec.MinReplicas
			}, 10*time.Second).Should(Equal(minReplicas))
		})
	})

	Context("Garbage Collection Integration Tests", func() {
		var testInstance *osrmv1alpha1.OSRMCluster

		AfterEach(func() {
			if testInstance != nil {
				Expect(k8sClient.Delete(ctx, testInstance)).To(Succeed())
				testInstance = nil
			}
		})

		It("should handle profile removal without duplicate deletion errors", func() {
			testInstance = generateOSRMCluster("gc-integration-test")

			By("Creating cluster with multiple profiles")
			// Add second profile
			osrmProfile := "foot"
			minReplicas := int32(1)
			maxReplicas := int32(2)
			walkingProfile := &osrmv1alpha1.ProfileSpec{
				Name:         "walking",
				EndpointName: "walking-endpoint",
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
			testInstance.Spec.Profiles = append(testInstance.Spec.Profiles, walkingProfile)

			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForDeployment(ctx, testInstance, k8sClient)

			By("Removing one profile")
			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.Profiles = []*osrmv1alpha1.ProfileSpec{v.Spec.Profiles[0]} // Keep only first profile
			})).To(Succeed())

			By("Verifying removed profile resources are cleaned up without errors")
			Eventually(func() bool {
				deployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName("walking", osrmResource.DeploymentSuffix),
					Namespace: testInstance.Namespace,
				}, deployment)
				return errors.IsNotFound(err)
			}, 30*time.Second).Should(BeTrue())

			By("Verifying reconciliation succeeds")
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

		It("should properly clean up old map generation resources", func() {
			testInstance = generateOSRMCluster("gc-map-generations")

			By("Creating initial cluster")
			Expect(k8sClient.Create(ctx, testInstance)).To(Succeed())
			waitForDeployment(ctx, testInstance, k8sClient)

			By("Triggering multiple map generation updates")
			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.PBFURL = "https://download.geofabrik.de/australia-oceania/marshall-islands-250713.osm.pbf"
			})).To(Succeed())

			// Wait for update to complete
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
			}, MapBuildingTimeout).Should(Equal(osrmv1alpha1.PhaseWorkersRedeployed))

			Expect(updateWithRetry(testInstance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.PBFURL = "https://download.geofabrik.de/australia-oceania/marshall-islands-250715.osm.pbf"
			})).To(Succeed())

			// Wait for update to complete
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
			}, MapBuildingTimeout).Should(Equal(osrmv1alpha1.PhaseWorkersRedeployed))

			By("Verifying old map generation resources are cleaned up")
			// Check that only the latest map generation resources exist
			Eventually(func() bool {
				// Generation 1 should be cleaned up
				pvc := &corev1.PersistentVolumeClaim{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testInstance.ChildResourceName(testInstance.Spec.Profiles[0].Name, "1"),
					Namespace: testInstance.Namespace,
				}, pvc)
				return errors.IsNotFound(err)
			}, 30*time.Second).Should(BeTrue())
		})
	})
})

// Helper functions for integration tests
func generateMultiProfileOSRMCluster(name string) *osrmv1alpha1.OSRMCluster {
	cluster := generateOSRMCluster(name)

	// Add second profile
	osrmProfile := "foot"
	internalEndpoint := "walking"
	minReplicas := int32(1)
	maxReplicas := int32(2)

	walkingProfile := &osrmv1alpha1.ProfileSpec{
		Name:             osrmProfile,
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

	cluster.Spec.Profiles = append(cluster.Spec.Profiles, walkingProfile)

	return cluster
}

func generateOSRMCluster(name string) *osrmv1alpha1.OSRMCluster {
	storage := resource.MustParse("10Mi")
	accessMode := corev1.ReadWriteOnce
	minReplicas := int32(1)
	maxReplicas := int32(3)

	serialNumber := atomic.AddInt64(&integrationTestCounter, 1)
	uniqueName := fmt.Sprintf("%s-%d", name, serialNumber)

	return &osrmv1alpha1.OSRMCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uniqueName,
			Namespace: "default",
		},
		Spec: osrmv1alpha1.OSRMClusterSpec{
			PBFURL: "https://download.geofabrik.de/australia-oceania/marshall-islands-latest.osm.pbf",
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

func waitForDeployment(ctx context.Context, instance *osrmv1alpha1.OSRMCluster, client client.Client) {
	EventuallyWithOffset(1, func() string {
		instanceCreated := osrmv1alpha1.OSRMCluster{}
		if err := k8sClient.Get(
			ctx,
			types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace},
			&instanceCreated,
		); err != nil {
			return fmt.Sprintf("%v+", err)
		}

		for _, condition := range instanceCreated.Status.Conditions {
			if condition.Type == status.ConditionAvailable && condition.Status == metav1.ConditionTrue {
				return "ready"
			}
		}

		return "not ready"
	}, MapBuildingTimeout, 1*time.Second).Should(Equal("ready"))
}
