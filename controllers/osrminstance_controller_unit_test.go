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

var _ = Describe("OSRMInstanceController Unit Tests", func() {
	var (
		reconciler *controllers.OSRMInstanceReconciler
	)

	BeforeEach(func() {
		s := scheme.Scheme
		err := osrmv1alpha1.AddToScheme(s)
		Expect(err).NotTo(HaveOccurred())

		fakeClient := fake.NewClientBuilder().WithScheme(s).Build()
		reconciler = controllers.NewOSRMInstanceReconciler(fakeClient, s)
	})

	Context("Phase Determination Tests", func() {
		var (
			instance   *osrmv1alpha1.OSRMInstance
			oldSpec    *osrmv1alpha1.OSRMInstanceSpec
			pvcs       []*corev1.PersistentVolumeClaim
			jobs       []*batchv1.Job
			deployment *appsv1.Deployment
		)

		BeforeEach(func() {
			instance = generateTestOSRMInstance("test-instance")
			oldSpec = &osrmv1alpha1.OSRMInstanceSpec{
				PBFURL:      instance.Spec.PBFURL,
				OSRMProfile: instance.Spec.OSRMProfile,
			}
			pvcs = []*corev1.PersistentVolumeClaim{}
			jobs = []*batchv1.Job{}
			deployment = nil
		})

		It("should return PhaseBuildingMap when no PVC exists", func() {
			phase := reconciler.DeterminePhase(instance, oldSpec, "1", "1", pvcs, jobs, deployment)

			Expect(phase).To(Equal(osrmv1alpha1.PhaseBuildingMap))
		})

		It("should return PhaseBuildingMap when PVC is pending", func() {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance.ChildResourceName("", "1"),
					Namespace: instance.Namespace,
				},
				Status: corev1.PersistentVolumeClaimStatus{
					Phase: corev1.ClaimPending,
				},
			}
			pvcs = append(pvcs, pvc)

			phase := reconciler.DeterminePhase(instance, oldSpec, "1", "1", pvcs, jobs, deployment)

			Expect(phase).To(Equal(osrmv1alpha1.PhaseBuildingMap))
		})

		It("should return PhaseUpdatingMap when PBF URL changes", func() {
			pvc := createBoundInstancePVC(instance, "1")
			job := createCompletedInstanceJob(instance, "1")
			pvcs = append(pvcs, pvc)
			jobs = append(jobs, job)
			oldSpec.PBFURL = "https://old-url.com/file.osm.pbf"
			instance.Spec.PBFURL = "https://new-url.com/file.osm.pbf"

			phase := reconciler.DeterminePhase(instance, oldSpec, "1", "1", pvcs, jobs, deployment)

			Expect(phase).To(Equal(osrmv1alpha1.PhaseUpdatingMap))
		})

		It("should return PhaseRedepoloyingWorkers when active and future map generations differ", func() {
			currentPVC := createBoundInstancePVC(instance, "1")
			currentJob := createCompletedInstanceJob(instance, "1")
			futurePVC := createBoundInstancePVC(instance, "2")
			futureJob := createCompletedInstanceJob(instance, "2")
			pvcs = append(pvcs, currentPVC, futurePVC)
			jobs = append(jobs, currentJob, futureJob)

			phase := reconciler.DeterminePhase(instance, oldSpec, "1", "2", pvcs, jobs, deployment)

			Expect(phase).To(Equal(osrmv1alpha1.PhaseRedepoloyingWorkers))
		})

		It("should return PhaseDeployingWorkers when deployment is not ready", func() {
			pvc := createBoundInstancePVC(instance, "1")
			job := createCompletedInstanceJob(instance, "1")
			pvcs = append(pvcs, pvc)
			jobs = append(jobs, job)

			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance.ChildResourceName("", osrmResource.DeploymentSuffix),
					Namespace: instance.Namespace,
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas: 0,
					Replicas:      1,
				},
			}

			phase := reconciler.DeterminePhase(instance, oldSpec, "1", "1", pvcs, jobs, deployment)

			Expect(phase).To(Equal(osrmv1alpha1.PhaseDeployingWorkers))
		})

		It("should return PhaseWorkersDeployed when deployment is ready and generation is 1", func() {
			pvc := createBoundInstancePVC(instance, "1")
			job := createCompletedInstanceJob(instance, "1")
			pvcs = append(pvcs, pvc)
			jobs = append(jobs, job)

			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance.ChildResourceName("", osrmResource.DeploymentSuffix),
					Namespace: instance.Namespace,
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas: 1,
					Replicas:      1,
				},
			}

			phase := reconciler.DeterminePhase(instance, oldSpec, "1", "1", pvcs, jobs, deployment)

			Expect(phase).To(Equal(osrmv1alpha1.PhaseWorkersDeployed))
		})

		It("should return PhaseWorkersRedeployed when deployment is ready and generation is 2", func() {
			pvc := createBoundInstancePVC(instance, "2")
			job := createCompletedInstanceJob(instance, "2")
			pvcs = append(pvcs, pvc)
			jobs = append(jobs, job)

			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance.ChildResourceName("", osrmResource.DeploymentSuffix),
					Namespace: instance.Namespace,
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: instance.ChildResourceName("", "2"),
										},
									},
								},
							},
						},
					},
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas: 1,
					Replicas:      1,
				},
			}

			phase := reconciler.DeterminePhase(instance, oldSpec, "2", "2", pvcs, jobs, deployment)

			Expect(phase).To(Equal(osrmv1alpha1.PhaseWorkersRedeployed))
		})
	})

	Context("Resource Name Generation Tests", func() {
		var instance *osrmv1alpha1.OSRMInstance

		BeforeEach(func() {
			instance = generateTestOSRMInstance("test-instance")
		})

		It("should generate correct resource names with empty component", func() {
			mapGeneration := "1"

			deploymentName := instance.ChildResourceName("", osrmResource.DeploymentSuffix)
			serviceName := instance.ChildResourceName("", osrmResource.ServiceSuffix)
			pvcName := instance.ChildResourceName("", mapGeneration)
			jobName := instance.ChildResourceName("", mapGeneration)
			cronJobName := instance.ChildResourceName("", osrmResource.CronJobSuffix)
			hpaName := instance.ChildResourceName("", osrmResource.HorizontalPodAutoscalerSuffix)
			pdbName := instance.ChildResourceName("", osrmResource.PodDisruptionBudgetSuffix)

			// Suffixes that are empty string collapse — ChildResourceName("", "") == instance.Name
			Expect(deploymentName).To(Equal(instance.Name))
			Expect(serviceName).To(Equal(instance.Name))
			Expect(hpaName).To(Equal(instance.Name))
			Expect(pdbName).To(Equal(instance.Name))
			// Non-empty suffixes produce "name-suffix"
			Expect(pvcName).To(Equal(fmt.Sprintf("%s-%s", instance.Name, mapGeneration)))
			Expect(jobName).To(Equal(fmt.Sprintf("%s-%s", instance.Name, mapGeneration)))
			Expect(cronJobName).To(Equal(fmt.Sprintf("%s-%s", instance.Name, osrmResource.CronJobSuffix)))
		})

		It("should handle names with dashes", func() {
			instance.Name = "my-osrm-instance-123"

			deploymentName := instance.ChildResourceName("", osrmResource.DeploymentSuffix)

			Expect(deploymentName).To(Equal("my-osrm-instance-123"))
		})
	})

	Context("Map Generation Logic Tests", func() {
		It("should return active map generation from deployment PVC claim name", func() {
			instance := generateTestOSRMInstance("test-instance")

			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance.ChildResourceName("", osrmResource.DeploymentSuffix),
					Namespace: instance.Namespace,
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: instance.ChildResourceName("", "2"),
										},
									},
								},
							},
						},
					},
				},
			}

			activeMapGeneration, err := reconciler.GetActiveMapGeneration(deployment)

			Expect(err).NotTo(HaveOccurred())
			Expect(activeMapGeneration).To(Equal("2"))
		})

		It("should return default active map generation when deployment is nil", func() {
			activeMapGeneration, err := reconciler.GetActiveMapGeneration(nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(activeMapGeneration).To(Equal("1"))
		})

		It("should return default active map generation when deployment has no volumes", func() {
			instance := generateTestOSRMInstance("test-instance")

			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance.ChildResourceName("", osrmResource.DeploymentSuffix),
					Namespace: instance.Namespace,
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{},
						},
					},
				},
			}

			activeMapGeneration, err := reconciler.GetActiveMapGeneration(deployment)

			Expect(err).NotTo(HaveOccurred())
			Expect(activeMapGeneration).To(Equal("1"))
		})

		It("should return correct future map generation from PVCs", func() {
			instance := generateTestOSRMInstance("test-instance")

			pvc1 := createBoundInstancePVC(instance, "1")
			pvc2 := createBoundInstancePVC(instance, "3")
			pvc3 := createBoundInstancePVC(instance, "2")

			pvcs := []*corev1.PersistentVolumeClaim{pvc1, pvc2, pvc3}

			futureMapGeneration, err := reconciler.GetFutureMapGeneration(pvcs)

			Expect(err).NotTo(HaveOccurred())
			Expect(futureMapGeneration).To(Equal("3"))
		})

		It("should return default future map generation when no PVCs exist", func() {
			pvcs := []*corev1.PersistentVolumeClaim{}

			futureMapGeneration, err := reconciler.GetFutureMapGeneration(pvcs)

			Expect(err).NotTo(HaveOccurred())
			Expect(futureMapGeneration).To(Equal("1"))
		})
	})

	Context("Garbage Collection Tests", func() {
		var (
			fakeClient client.Client
			instance   *osrmv1alpha1.OSRMInstance
			ctx        context.Context
		)

		BeforeEach(func() {
			ctx = context.Background()
			s := scheme.Scheme
			err := osrmv1alpha1.AddToScheme(s)
			Expect(err).NotTo(HaveOccurred())

			fakeClient = fake.NewClientBuilder().WithScheme(s).Build()
			reconciler = controllers.NewOSRMInstanceReconciler(fakeClient, s)

			instance = generateTestOSRMInstance("gc-test-instance")
		})

		It("should handle multiple garbage collection runs without errors", func() {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance.ChildResourceName("", osrmResource.DeploymentSuffix),
					Namespace: instance.Namespace,
					Labels:    map[string]string{"osrm.itayankri/name": instance.Name},
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "data",
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: instance.ChildResourceName("", "1"),
											ReadOnly:  true,
										},
									},
								},
							},
						},
					},
				},
			}
			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())

			for i := 0; i < 3; i++ {
				err := reconciler.GarbageCollection(ctx, instance)
				Expect(err).NotTo(HaveOccurred())
			}

			err := fakeClient.Get(ctx, types.NamespacedName{
				Name:      instance.ChildResourceName("", osrmResource.DeploymentSuffix),
				Namespace: instance.Namespace,
			}, deployment)
			Expect(err).NotTo(HaveOccurred())
		})

		XIt("should not delete expected resources during garbage collection", func() {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance.ChildResourceName("", "1"),
					Namespace: instance.Namespace,
					Labels:    map[string]string{"osrm.itayankri/name": instance.Name},
				},
			}
			Expect(fakeClient.Create(ctx, pvc)).To(Succeed())

			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance.ChildResourceName("", "1"),
					Namespace: instance.Namespace,
					Labels:    map[string]string{"osrm.itayankri/name": instance.Name},
				},
			}
			Expect(fakeClient.Create(ctx, job)).To(Succeed())

			err := reconciler.GarbageCollection(ctx, instance)
			Expect(err).NotTo(HaveOccurred())

			gotPVC := &corev1.PersistentVolumeClaim{}
			err = fakeClient.Get(ctx, types.NamespacedName{
				Name:      instance.ChildResourceName("", "1"),
				Namespace: instance.Namespace,
			}, gotPVC)
			Expect(errors.IsNotFound(err)).To(BeFalse())
		})
	})
})

func generateTestOSRMInstance(name string) *osrmv1alpha1.OSRMInstance {
	storage := resource.MustParse("10Mi")
	accessMode := corev1.ReadWriteOnce
	minReplicas := int32(1)
	maxReplicas := int32(3)

	serialNumber := atomic.AddInt64(&unitTestCounter, 1)
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
				Storage:          &storage,
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

func createBoundInstancePVC(instance *osrmv1alpha1.OSRMInstance, mapGeneration string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.ChildResourceName("", mapGeneration),
			Namespace: instance.Namespace,
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
		},
	}
}

func createCompletedInstanceJob(instance *osrmv1alpha1.OSRMInstance, mapGeneration string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.ChildResourceName("", mapGeneration),
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
