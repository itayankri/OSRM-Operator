package resource_test

import (
	"fmt"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/resource"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("Job builder", func() {
	var (
		scheme     *runtime.Scheme
		jobBuilder resource.ResourceBuilder
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(osrmv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())
		jobBuilder = osrmResourceBuilder.Job(instance.Spec.Profiles[0], "v1")
	})

	Context("Build", func() {
		It("Should use values from custom resource", func() {
			obj, err := jobBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			job := obj.(*batchv1.Job)

			By("generates a job object with the correct name", func() {
				expectedName := fmt.Sprintf("%s-%s-%s", instance.ObjectMeta.Name, instance.Spec.Profiles[0].Name, "v1")
				Expect(job.Name).To(Equal(expectedName))
			})

			By("generates a job object with the correct namespace", func() {
				Expect(job.Namespace).To(Equal(instance.Namespace))
			})

			By("sets the default environment variables", func() {
				env := job.Spec.Template.Spec.Containers[0].Env
				Expect(env).To(ContainElement(corev1.EnvVar{Name: "ROOT_DIR", Value: "/data"}))
				Expect(env).To(ContainElement(corev1.EnvVar{Name: "PARTITIONED_DATA_DIR", Value: "partitioned"}))
				Expect(env).To(ContainElement(corev1.EnvVar{Name: "CUSTOMIZED_DATA_DIR", Value: "customized"}))
				Expect(env).To(ContainElement(corev1.EnvVar{Name: "PBF_URL", Value: instance.Spec.PBFURL}))
				Expect(env).To(ContainElement(corev1.EnvVar{Name: "PROFILE", Value: instance.Spec.Profiles[0].GetProfile()}))
			})
		})

		It("Should include custom environment variables when MapBuilder.Env is set", func() {
			// Create a custom instance with environment variables
			customInstance := instance.DeepCopy()
			customInstance.Spec.MapBuilder.Env = []corev1.EnvVar{
				{Name: "CUSTOM_VAR_1", Value: "value1"},
				{Name: "CUSTOM_VAR_2", Value: "value2"},
				{
					Name: "SECRET_VAR",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
							Key:                  "secret-key",
						},
					},
				},
			}

			customBuilder := &resource.OSRMResourceBuilder{
				Instance: customInstance,
			}
			jobBuilder := customBuilder.Job(customInstance.Spec.Profiles[0], "v1")

			obj, err := jobBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			job := obj.(*batchv1.Job)

			By("includes the custom environment variables", func() {
				env := job.Spec.Template.Spec.Containers[0].Env
				Expect(env).To(ContainElement(corev1.EnvVar{Name: "CUSTOM_VAR_1", Value: "value1"}))
				Expect(env).To(ContainElement(corev1.EnvVar{Name: "CUSTOM_VAR_2", Value: "value2"}))
			})

			By("includes environment variables from secrets", func() {
				env := job.Spec.Template.Spec.Containers[0].Env
				var secretVar corev1.EnvVar
				for _, e := range env {
					if e.Name == "SECRET_VAR" {
						secretVar = e
						break
					}
				}
				Expect(secretVar.Name).To(Equal("SECRET_VAR"))
				Expect(secretVar.ValueFrom).NotTo(BeNil())
				Expect(secretVar.ValueFrom.SecretKeyRef).NotTo(BeNil())
				Expect(secretVar.ValueFrom.SecretKeyRef.Name).To(Equal("my-secret"))
				Expect(secretVar.ValueFrom.SecretKeyRef.Key).To(Equal("secret-key"))
			})

			By("retains all default environment variables", func() {
				env := job.Spec.Template.Spec.Containers[0].Env
				Expect(env).To(ContainElement(corev1.EnvVar{Name: "ROOT_DIR", Value: "/data"}))
				Expect(env).To(ContainElement(corev1.EnvVar{Name: "PBF_URL", Value: customInstance.Spec.PBFURL}))
				Expect(env).To(ContainElement(corev1.EnvVar{Name: "PROFILE", Value: customInstance.Spec.Profiles[0].GetProfile()}))
			})
		})

		It("Should handle nil MapBuilder.Env gracefully", func() {
			// Create a custom instance with nil environment variables
			customInstance := instance.DeepCopy()
			customInstance.Spec.MapBuilder.Env = nil

			customBuilder := &resource.OSRMResourceBuilder{
				Instance: customInstance,
			}
			jobBuilder := customBuilder.Job(customInstance.Spec.Profiles[0], "v1")

			obj, err := jobBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			job := obj.(*batchv1.Job)

			By("only includes the default environment variables", func() {
				env := job.Spec.Template.Spec.Containers[0].Env
				Expect(env).To(ContainElement(corev1.EnvVar{Name: "ROOT_DIR", Value: "/data"}))
				Expect(env).To(ContainElement(corev1.EnvVar{Name: "PBF_URL", Value: customInstance.Spec.PBFURL}))
			})
		})

		It("Should handle empty MapBuilder.Env slice gracefully", func() {
			// Create a custom instance with empty environment variables
			customInstance := instance.DeepCopy()
			customInstance.Spec.MapBuilder.Env = []corev1.EnvVar{}

			customBuilder := &resource.OSRMResourceBuilder{
				Instance: customInstance,
			}
			jobBuilder := customBuilder.Job(customInstance.Spec.Profiles[0], "v1")

			obj, err := jobBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			job := obj.(*batchv1.Job)

			By("only includes the default environment variables", func() {
				env := job.Spec.Template.Spec.Containers[0].Env
				Expect(env).To(ContainElement(corev1.EnvVar{Name: "ROOT_DIR", Value: "/data"}))
				Expect(env).To(ContainElement(corev1.EnvVar{Name: "PBF_URL", Value: customInstance.Spec.PBFURL}))
			})
		})

		It("Should append custom env vars after default env vars", func() {
			// Create a custom instance with environment variables
			customInstance := instance.DeepCopy()
			customInstance.Spec.MapBuilder.Env = []corev1.EnvVar{
				{Name: "CUSTOM_VAR", Value: "custom_value"},
			}

			customBuilder := &resource.OSRMResourceBuilder{
				Instance: customInstance,
			}
			jobBuilder := customBuilder.Job(customInstance.Spec.Profiles[0], "v1")

			obj, err := jobBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			job := obj.(*batchv1.Job)

			By("ensures custom env vars are appended, not prepended", func() {
				env := job.Spec.Template.Spec.Containers[0].Env
				// Find the indices
				var rootDirIndex, customVarIndex int
				for i, e := range env {
					if e.Name == "ROOT_DIR" {
						rootDirIndex = i
					}
					if e.Name == "CUSTOM_VAR" {
						customVarIndex = i
					}
				}
				// Custom var should come after the default vars
				Expect(customVarIndex).To(BeNumerically(">", rootDirIndex))
			})
		})
	})
})
