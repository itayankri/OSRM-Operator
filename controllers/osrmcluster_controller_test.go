package controllers_test

import (
	"context"
	"fmt"
	"time"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	osrmResource "github.com/itayankri/OSRM-Operator/internal/resource"
	"github.com/itayankri/OSRM-Operator/internal/status"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ClusterDeletionTimeout = 5 * time.Second
	MapBuildingTimeout     = 2 * 60 * time.Second
)

var instance *osrmv1alpha1.OSRMCluster
var defaultNamespace = "default"

var _ = Describe("OSRMClusterController", func() {
	Context("Resource requirements configurations", func() {
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, instance)).To(Succeed())
		})

		FIt("uses resource requirements from profile spec when provided", func() {
			instance = generateOSRMCluster("resource-requirements-config")
			expectedResources := corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			}
			instance.Spec.Profiles[0].Resources = &expectedResources
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())
			waitForDeployment(ctx, instance, k8sClient)
			deployment := deployment(ctx, instance.Name, instance.Spec.Profiles[0].Name, osrmResource.DeploymentSuffix)
			actualResources := deployment.Spec.Template.Spec.Containers[0].Resources
			Expect(actualResources).To(Equal(expectedResources))
		})
	})

	Context("Custom Resource updates", func() {
		BeforeEach(func() {
			instance = generateOSRMCluster("custom-resource-updates")
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())
			waitForDeployment(ctx, instance, k8sClient)
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, instance)).To(Succeed())
		})

		It("Should update deployment CPU and memory requests and limits", func() {
			var resourceRequirements corev1.ResourceRequirements
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

			Expect(updateWithRetry(instance, func(v *osrmv1alpha1.OSRMCluster) {
				v.Spec.Profiles[0].Resources = expectedRequirements
			})).To(Succeed())

			Eventually(func() corev1.ResourceList {
				deployment := deployment(ctx, instance.Name, instance.Spec.Profiles[0].Name, osrmResource.DeploymentSuffix)
				resourceRequirements = deployment.Spec.Template.Spec.Containers[0].Resources
				return resourceRequirements.Requests
			}, 3).Should(HaveKeyWithValue(corev1.ResourceCPU, expectedRequirements.Requests[corev1.ResourceCPU]))
			Expect(resourceRequirements.Limits).To(HaveKeyWithValue(corev1.ResourceCPU, expectedRequirements.Limits[corev1.ResourceCPU]))
			Expect(resourceRequirements.Requests).To(HaveKeyWithValue(corev1.ResourceMemory, expectedRequirements.Requests[corev1.ResourceMemory]))
			Expect(resourceRequirements.Limits).To(HaveKeyWithValue(corev1.ResourceMemory, expectedRequirements.Limits[corev1.ResourceMemory]))
		})
	})

	Context("Recreate child resources after deletion", func() {
		BeforeEach(func() {
			instance = generateOSRMCluster("recreate-children")
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())
			waitForDeployment(ctx, instance, k8sClient)
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, instance)).To(Succeed())
		})

		It("recreates child resources after deletion", func() {
			oldService := service(ctx, instance.Name, instance.Spec.Profiles[0].Name, osrmResource.ServiceSuffix)
			oldDeployment := deployment(ctx, instance.Name, instance.Spec.Profiles[0].Name, osrmResource.DeploymentSuffix)
			oldHpa := hpa(ctx, instance.Name, instance.Spec.Profiles[0].Name, osrmResource.HorizontalPodAutoscalerSuffix)

			Expect(k8sClient.Delete(ctx, oldService)).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, oldHpa)).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, oldDeployment)).NotTo(HaveOccurred())

			Eventually(func() bool {
				deployment := deployment(ctx, instance.Name, instance.Spec.Profiles[0].Name, osrmResource.DeploymentSuffix)
				return string(deployment.UID) != string(oldDeployment.UID)
			}, 5).Should(BeTrue())

			Eventually(func() bool {
				svc := service(ctx, instance.Name, instance.Spec.Profiles[0].Name, osrmResource.ServiceSuffix)
				return string(svc.UID) != string(oldService.UID)
			}, 5).Should(BeTrue())

			Eventually(func() bool {
				hpa := hpa(ctx, instance.Name, instance.Spec.Profiles[0].Name, osrmResource.HorizontalPodAutoscalerSuffix)
				return string(hpa.UID) != string(oldHpa.UID)
			}, 5).Should(BeTrue())
		})
	})

	Context("OSRMCluster CR ReconcileSuccess condition", func() {
		BeforeEach(func() {
			instance = generateOSRMCluster("reconcile-success-condition")
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, instance)).To(Succeed())
		})

		It("Should keep ReconcileSuccess condition updated", func() {
			By("setting to False when spec is not valid", func() {
				// It is impossible to create a deployment with -1 replicas. Thus we expect reconcilication to fail.
				instance.Spec.Profiles[0].MinReplicas = pointer.Int32Ptr(-1)
				Expect(k8sClient.Create(ctx, instance)).To(Succeed())
				waitForOSRMClusterCreation(ctx, instance, k8sClient)

				Eventually(func() metav1.ConditionStatus {
					osrmCluster := &osrmv1alpha1.OSRMCluster{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{
						Name:      instance.Name,
						Namespace: instance.Namespace,
					}, osrmCluster)).To(Succeed())

					for _, condition := range osrmCluster.Status.Conditions {
						if condition.Type == status.ConditionReconciliationSuccess {
							return condition.Status
						}
					}
					return metav1.ConditionUnknown
				}, 60*time.Second).Should(Equal(metav1.ConditionFalse))
			})

			By("setting to True when spec is valid", func() {
				// It is impossible to create a deployment with -1 replicas. Thus we expect reconcilication to fail.
				Expect(updateWithRetry(instance, func(v *osrmv1alpha1.OSRMCluster) {
					v.Spec.Profiles[0].MinReplicas = pointer.Int32Ptr(2)
				})).To(Succeed())

				Eventually(func() metav1.ConditionStatus {
					osrmCluster := &osrmv1alpha1.OSRMCluster{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{
						Name:      instance.Name,
						Namespace: instance.Namespace,
					}, osrmCluster)).To(Succeed())

					for _, condition := range osrmCluster.Status.Conditions {
						if condition.Type == status.ConditionReconciliationSuccess {
							return condition.Status
						}
					}
					return metav1.ConditionUnknown
				}, 60*time.Second).Should(Equal(metav1.ConditionTrue))
			})
		})
	})

	Context("Pause reconciliation", func() {
		BeforeEach(func() {
			instance = generateOSRMCluster("pause-reconcile")
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())
			waitForDeployment(ctx, instance, k8sClient)
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, instance)).To(Succeed())
		})

		It("Should skip OSRMCluster if pause reconciliation annotation is set to true", func() {
			minReplicas := int32(2)
			originalMinReplicas := *instance.Spec.Profiles[0].MinReplicas
			Expect(updateWithRetry(instance, func(v *osrmv1alpha1.OSRMCluster) {
				v.SetAnnotations(map[string]string{"osrm.itayankri/operator.paused": "true"})
				v.Spec.Profiles[0].MinReplicas = &minReplicas
			})).To(Succeed())

			Eventually(func() int32 {
				return *hpa(ctx, instance.Name, instance.Spec.Profiles[0].Name, osrmResource.HorizontalPodAutoscalerSuffix).Spec.MinReplicas
			}, MapBuildingTimeout).Should(Equal(originalMinReplicas))

			Expect(updateWithRetry(instance, func(v *osrmv1alpha1.OSRMCluster) {
				v.SetAnnotations(map[string]string{"osrm.itayankri/operator.paused": "false"})
			})).To(Succeed())

			Eventually(func() int32 {
				return *hpa(ctx, instance.Name, instance.Spec.Profiles[0].Name, osrmResource.HorizontalPodAutoscalerSuffix).Spec.MinReplicas
			}, 10*time.Second).Should(Equal(minReplicas))
		})
	})
})

func generateOSRMCluster(name string) *osrmv1alpha1.OSRMCluster {
	storage := resource.MustParse("10Mi")
	image := "osrm/osrm-backend"
	minReplicas := int32(1)
	maxReplicas := int32(3)
	osrmCluster := &osrmv1alpha1.OSRMCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: defaultNamespace,
		},
		Spec: osrmv1alpha1.OSRMClusterSpec{
			PBFURL: "https://download.geofabrik.de/australia-oceania/marshall-islands-latest.osm.pbf",
			Image:  &image,
			Persistence: osrmv1alpha1.PersistenceSpec{
				StorageClassName: "nfs-csi",
				Storage:          &storage,
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
		},
	}
	return osrmCluster
}

func waitForOSRMClusterCreation(ctx context.Context, instance *osrmv1alpha1.OSRMCluster, client client.Client) {
	EventuallyWithOffset(1, func() string {
		instanceCreated := osrmv1alpha1.OSRMCluster{}
		if err := k8sClient.Get(
			ctx,
			types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace},
			&instanceCreated,
		); err != nil {
			return fmt.Sprintf("%v+", err)
		}

		if len(instanceCreated.Status.Conditions) == 0 {
			return "not ready"
		}

		return "ready"

	}, MapBuildingTimeout, 1*time.Second).Should(Equal("ready"))
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

func hpa(ctx context.Context, clusterName string, profileName string, suffix string) *autoscalingv1.HorizontalPodAutoscaler {
	name := fmt.Sprintf("%s-%s", clusterName, profileName)
	if len(suffix) > 0 {
		name = fmt.Sprintf("%s-%s", name, suffix)
	}
	hpa := &autoscalingv1.HorizontalPodAutoscaler{}
	EventuallyWithOffset(1, func() error {
		if err := k8sClient.Get(
			ctx,
			types.NamespacedName{Name: name, Namespace: defaultNamespace},
			hpa,
		); err != nil {
			return err
		}
		return nil
	}, MapBuildingTimeout).Should(Succeed())
	return hpa
}

func service(ctx context.Context, clusterName string, profileName string, suffix string) *corev1.Service {
	name := fmt.Sprintf("%s-%s", clusterName, profileName)
	if len(suffix) > 0 {
		name = fmt.Sprintf("%s-%s", name, suffix)
	}
	svc := &corev1.Service{}
	EventuallyWithOffset(1, func() error {
		if err := k8sClient.Get(
			ctx,
			types.NamespacedName{Name: name, Namespace: defaultNamespace},
			svc,
		); err != nil {
			return err
		}
		return nil
	}, MapBuildingTimeout).Should(Succeed())
	return svc
}

func deployment(ctx context.Context, clusterName string, profileName string, suffix string) *appsv1.Deployment {
	name := fmt.Sprintf("%s-%s", clusterName, profileName)
	if len(suffix) > 0 {
		name = fmt.Sprintf("%s-%s", name, suffix)
	}
	deployment := &appsv1.Deployment{}
	EventuallyWithOffset(1, func() error {
		if err := k8sClient.Get(
			ctx,
			types.NamespacedName{Name: name, Namespace: defaultNamespace},
			deployment,
		); err != nil {
			return err
		}
		return nil
	}, MapBuildingTimeout).Should(Succeed())
	return deployment
}
