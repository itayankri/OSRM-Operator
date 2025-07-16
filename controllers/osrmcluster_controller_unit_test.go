package controllers_test

import (
	"context"
	"fmt"
	"sync/atomic"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/controllers"
	osrmResource "github.com/itayankri/OSRM-Operator/internal/resource"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var unitTestCounter int64

var _ = Describe("OSRMClusterController Unit Tests", func() {
	var (
		reconciler *controllers.OSRMClusterReconciler
	)

	BeforeEach(func() {
		s := scheme.Scheme
		err := osrmv1alpha1.AddToScheme(s)
		Expect(err).NotTo(HaveOccurred())

		fakeClient := fake.NewClientBuilder().WithScheme(s).Build()
		reconciler = controllers.NewOSRMClusterReconciler(fakeClient, s)
	})

	Context("Phase Determination Tests", func() {
		var (
			instance            *osrmv1alpha1.OSRMCluster
			oldSpec             *osrmv1alpha1.OSRMClusterSpec
			pvcs                []*corev1.PersistentVolumeClaim
			jobs                []*batchv1.Job
			profilesDeployments []*appsv1.Deployment
		)

		BeforeEach(func() {
			instance = generateTestOSRMCluster("test-cluster")
			oldSpec = &osrmv1alpha1.OSRMClusterSpec{
				PBFURL:   instance.Spec.PBFURL,
				Profiles: instance.Spec.Profiles,
			}
			pvcs = []*corev1.PersistentVolumeClaim{}
			jobs = []*batchv1.Job{}
			profilesDeployments = []*appsv1.Deployment{}
		})

		It("should return PhaseEmpty when no profiles are specified", func() {
			instance.Spec.Profiles = []*osrmv1alpha1.ProfileSpec{}

			phase := reconciler.DeterminePhase(instance, oldSpec, "1", "1", pvcs, jobs, profilesDeployments)

			Expect(phase).To(Equal(osrmv1alpha1.PhaseEmpty))
		})

		It("should return PhaseBuildingMap when map building is in progress", func() {
			// Create PVC that is not bound
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance.ChildResourceName(instance.Spec.Profiles[0].Name, "1"),
					Namespace: instance.Namespace,
				},
				Status: corev1.PersistentVolumeClaimStatus{
					Phase: corev1.ClaimPending,
				},
			}
			pvcs = append(pvcs, pvc)

			phase := reconciler.DeterminePhase(instance, oldSpec, "1", "1", pvcs, jobs, profilesDeployments)

			Expect(phase).To(Equal(osrmv1alpha1.PhaseBuildingMap))
		})

		It("should return PhaseUpdatingMap when PBF URL changes", func() {
			oldSpec.PBFURL = "https://old-url.com/file.pbf"
			instance.Spec.PBFURL = "https://new-url.com/file.pbf"

			phase := reconciler.DeterminePhase(instance, oldSpec, "1", "1", pvcs, jobs, profilesDeployments)

			Expect(phase).To(Equal(osrmv1alpha1.PhaseUpdatingMap))
		})

		It("should return PhaseRedepoloyingWorkers when active and future map generations differ", func() {
			// Create bound PVC and completed job for generation 1
			pvc := createBoundPVC(instance, "1")
			job := createCompletedJob(instance, "1")
			pvcs = append(pvcs, pvc)
			jobs = append(jobs, job)

			phase := reconciler.DeterminePhase(instance, oldSpec, "1", "2", pvcs, jobs, profilesDeployments)

			Expect(phase).To(Equal(osrmv1alpha1.PhaseRedepoloyingWorkers))
		})

		It("should return PhaseDeployingWorkers when workers are not ready", func() {
			// Create bound PVC and completed job
			pvc := createBoundPVC(instance, "1")
			job := createCompletedJob(instance, "1")
			pvcs = append(pvcs, pvc)
			jobs = append(jobs, job)

			// Create deployment that is not ready
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance.ChildResourceName(instance.Spec.Profiles[0].Name, osrmResource.DeploymentSuffix),
					Namespace: instance.Namespace,
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas: 0,
					Replicas:      1,
				},
			}
			profilesDeployments = append(profilesDeployments, deployment)

			phase := reconciler.DeterminePhase(instance, oldSpec, "1", "1", pvcs, jobs, profilesDeployments)

			Expect(phase).To(Equal(osrmv1alpha1.PhaseDeployingWorkers))
		})

		It("should return PhaseWorkersDeployed when all workers are ready", func() {
			// Create bound PVC and completed job
			pvc := createBoundPVC(instance, "1")
			job := createCompletedJob(instance, "1")
			pvcs = append(pvcs, pvc)
			jobs = append(jobs, job)

			// Create deployment that is ready
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance.ChildResourceName(instance.Spec.Profiles[0].Name, osrmResource.DeploymentSuffix),
					Namespace: instance.Namespace,
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas: 1,
					Replicas:      1,
				},
			}
			profilesDeployments = append(profilesDeployments, deployment)

			phase := reconciler.DeterminePhase(instance, oldSpec, "1", "1", pvcs, jobs, profilesDeployments)

			Expect(phase).To(Equal(osrmv1alpha1.PhaseWorkersDeployed))
		})

		It("should return PhaseWorkersRedeployed when workers are ready after redeployment", func() {
			// Create bound PVC and completed job
			pvc := createBoundPVC(instance, "2")
			job := createCompletedJob(instance, "2")
			pvcs = append(pvcs, pvc)
			jobs = append(jobs, job)

			// Create deployment that is ready
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance.ChildResourceName(instance.Spec.Profiles[0].Name, osrmResource.DeploymentSuffix),
					Namespace: instance.Namespace,
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas: 1,
					Replicas:      1,
				},
			}
			profilesDeployments = append(profilesDeployments, deployment)

			phase := reconciler.DeterminePhase(instance, oldSpec, "2", "2", pvcs, jobs, profilesDeployments)

			Expect(phase).To(Equal(osrmv1alpha1.PhaseWorkersRedeployed))
		})
	})

	Context("Resource Name Generation Tests", func() {
		var instance *osrmv1alpha1.OSRMCluster

		BeforeEach(func() {
			instance = generateTestOSRMCluster("test-cluster")
		})

		It("should generate correct resource names for profiles", func() {
			profileName := "car"

			deploymentName := instance.ChildResourceName(profileName, osrmResource.DeploymentSuffix)
			serviceName := instance.ChildResourceName(profileName, osrmResource.ServiceSuffix)
			pvcName := instance.ChildResourceName(profileName, "1")
			jobName := instance.ChildResourceName(profileName, "1")
			cronJobName := instance.ChildResourceName(profileName, osrmResource.CronJobSuffix)

			Expect(deploymentName).To(Equal("test-cluster-car"))
			Expect(serviceName).To(Equal("test-cluster-car"))
			Expect(pvcName).To(Equal("test-cluster-car-1"))
			Expect(jobName).To(Equal("test-cluster-car-1"))
			Expect(cronJobName).To(Equal("test-cluster-car-speed-updates"))
		})

		It("should generate correct resource names for gateway", func() {
			gatewayDeploymentName := instance.ChildResourceName(osrmResource.GatewaySuffix, osrmResource.DeploymentSuffix)
			gatewayServiceName := instance.ChildResourceName(osrmResource.GatewaySuffix, osrmResource.ServiceSuffix)
			gatewayConfigMapName := instance.ChildResourceName(osrmResource.GatewaySuffix, osrmResource.ConfigMapSuffix)

			Expect(gatewayDeploymentName).To(Equal("test-cluster"))
			Expect(gatewayServiceName).To(Equal("test-cluster"))
			Expect(gatewayConfigMapName).To(Equal("test-cluster"))
		})

		It("should handle special characters in names", func() {
			instance.Name = "test-cluster-123"
			profileName := "car-driving"

			deploymentName := instance.ChildResourceName(profileName, osrmResource.DeploymentSuffix)

			Expect(deploymentName).To(Equal("test-cluster-123-car-driving"))
		})
	})

	Context("Map Generation Logic Tests", func() {
		It("should return correct active map generation from deployments", func() {
			instance := generateTestOSRMCluster("test-cluster")

			// Create deployment with PVC that has map generation 2
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance.ChildResourceName(instance.Spec.Profiles[0].Name, osrmResource.DeploymentSuffix),
					Namespace: instance.Namespace,
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: instance.ChildResourceName(instance.Spec.Profiles[0].Name, "2"),
										},
									},
								},
							},
						},
					},
				},
			}

			profilesDeployments := []*appsv1.Deployment{deployment}

			activeMapGeneration, err := reconciler.GetActiveMapGeneration(profilesDeployments)

			Expect(err).NotTo(HaveOccurred())
			Expect(activeMapGeneration).To(Equal("2"))
		})

		It("should return default active map generation when no deployments", func() {
			profilesDeployments := []*appsv1.Deployment{}

			activeMapGeneration, err := reconciler.GetActiveMapGeneration(profilesDeployments)

			Expect(err).NotTo(HaveOccurred())
			Expect(activeMapGeneration).To(Equal("1"))
		})

		It("should return correct future map generation from PVCs", func() {
			instance := generateTestOSRMCluster("test-cluster")

			// Create PVCs with different map generations
			pvc1 := createBoundPVC(instance, "1")
			pvc2 := createBoundPVC(instance, "3")
			pvc3 := createBoundPVC(instance, "2")

			pvcs := []*corev1.PersistentVolumeClaim{pvc1, pvc2, pvc3}

			futureMapGeneration, err := reconciler.GetFutureMapGeneration(pvcs)

			Expect(err).NotTo(HaveOccurred())
			Expect(futureMapGeneration).To(Equal("3"))
		})

		It("should return default future map generation when no PVCs", func() {
			pvcs := []*corev1.PersistentVolumeClaim{}

			futureMapGeneration, err := reconciler.GetFutureMapGeneration(pvcs)

			Expect(err).NotTo(HaveOccurred())
			Expect(futureMapGeneration).To(Equal("1"))
		})
	})

	Context("Garbage Collection Tests", func() {
		var (
			fakeClient client.Client
			instance   *osrmv1alpha1.OSRMCluster
			ctx        context.Context
		)

		BeforeEach(func() {
			ctx = context.Background()
			s := scheme.Scheme
			err := osrmv1alpha1.AddToScheme(s)
			Expect(err).NotTo(HaveOccurred())

			fakeClient = fake.NewClientBuilder().WithScheme(s).Build()
			reconciler = controllers.NewOSRMClusterReconciler(fakeClient, s)

			instance = generateTestOSRMCluster("gc-test-cluster")
		})

		It("should clean up resources for removed profiles", func() {
			// Setup: Create resources for multiple profiles
			instance.Spec.Profiles = []*osrmv1alpha1.ProfileSpec{
				createTestProfile("car", "driving"),
				createTestProfile("walking", "walking"),
				createTestProfile("cycling", "cycling"),
			}

			// Create profile-scoped resources
			for _, profile := range instance.Spec.Profiles {
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      instance.ChildResourceName(profile.Name, osrmResource.DeploymentSuffix),
						Namespace: instance.Namespace,
						Labels:    map[string]string{"osrm.itayankri/name": instance.Name},
					},
				}
				Expect(fakeClient.Create(ctx, deployment)).To(Succeed())

				service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      instance.ChildResourceName(profile.Name, osrmResource.ServiceSuffix),
						Namespace: instance.Namespace,
						Labels:    map[string]string{"osrm.itayankri/name": instance.Name},
					},
				}
				Expect(fakeClient.Create(ctx, service)).To(Succeed())

				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      instance.ChildResourceName(profile.Name, "1"),
						Namespace: instance.Namespace,
						Labels:    map[string]string{"osrm.itayankri/name": instance.Name},
					},
				}
				Expect(fakeClient.Create(ctx, pvc)).To(Succeed())

				job := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      instance.ChildResourceName(profile.Name, "1"),
						Namespace: instance.Namespace,
						Labels:    map[string]string{"osrm.itayankri/name": instance.Name},
					},
				}
				Expect(fakeClient.Create(ctx, job)).To(Succeed())
			}

			// Remove two profiles (keep only car)
			instance.Spec.Profiles = []*osrmv1alpha1.ProfileSpec{
				createTestProfile("car", "driving"),
			}

			// Run garbage collection
			err := reconciler.GarbageCollection(ctx, instance)
			Expect(err).NotTo(HaveOccurred())

			// Verify removed profile resources are deleted
			for _, profileName := range []string{"walking", "cycling"} {
				deployment := &appsv1.Deployment{}
				err := fakeClient.Get(ctx, types.NamespacedName{
					Name:      instance.ChildResourceName(profileName, osrmResource.DeploymentSuffix),
					Namespace: instance.Namespace,
				}, deployment)
				Expect(errors.IsNotFound(err)).To(BeTrue())

				service := &corev1.Service{}
				err = fakeClient.Get(ctx, types.NamespacedName{
					Name:      instance.ChildResourceName(profileName, osrmResource.ServiceSuffix),
					Namespace: instance.Namespace,
				}, service)
				Expect(errors.IsNotFound(err)).To(BeTrue())

				pvc := &corev1.PersistentVolumeClaim{}
				err = fakeClient.Get(ctx, types.NamespacedName{
					Name:      instance.ChildResourceName(profileName, "1"),
					Namespace: instance.Namespace,
				}, pvc)
				Expect(errors.IsNotFound(err)).To(BeTrue())

				job := &batchv1.Job{}
				err = fakeClient.Get(ctx, types.NamespacedName{
					Name:      instance.ChildResourceName(profileName, "1"),
					Namespace: instance.Namespace,
				}, job)
				Expect(errors.IsNotFound(err)).To(BeTrue())
			}

			// Verify remaining profile resources are preserved
			deployment := &appsv1.Deployment{}
			err = fakeClient.Get(ctx, types.NamespacedName{
				Name:      instance.ChildResourceName("car", osrmResource.DeploymentSuffix),
				Namespace: instance.Namespace,
			}, deployment)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should handle multiple garbage collection runs without errors", func() {
			// Setup: Create a resource
			instance.Spec.Profiles = []*osrmv1alpha1.ProfileSpec{
				createTestProfile("car", "driving"),
			}

			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance.ChildResourceName("car", osrmResource.DeploymentSuffix),
					Namespace: instance.Namespace,
					Labels:    map[string]string{"osrm.itayankri/name": instance.Name},
				},
			}
			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())

			// Run garbage collection multiple times
			for i := 0; i < 3; i++ {
				err := reconciler.GarbageCollection(ctx, instance)
				Expect(err).NotTo(HaveOccurred())
			}

			// Verify resource still exists
			err := fakeClient.Get(ctx, types.NamespacedName{
				Name:      instance.ChildResourceName("car", osrmResource.DeploymentSuffix),
				Namespace: instance.Namespace,
			}, deployment)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

// Helper functions
func generateTestOSRMCluster(name string) *osrmv1alpha1.OSRMCluster {
	storage := resource.MustParse("10Mi")
	accessMode := corev1.ReadWriteOnce
	minReplicas := int32(1)
	maxReplicas := int32(3)

	serialNumber := atomic.AddInt64(&unitTestCounter, 1)
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

func createBoundPVC(instance *osrmv1alpha1.OSRMCluster, mapGeneration string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.ChildResourceName(instance.Spec.Profiles[0].Name, mapGeneration),
			Namespace: instance.Namespace,
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
		},
	}
}

func createCompletedJob(instance *osrmv1alpha1.OSRMCluster, mapGeneration string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.ChildResourceName(instance.Spec.Profiles[0].Name, mapGeneration),
			Namespace: instance.Namespace,
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}
