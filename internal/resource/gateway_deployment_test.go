package resource_test

import (
	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/resource"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("GatewayDeployment builder", func() {
	var scheme *runtime.Scheme

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(osrmv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())
	})

	buildGatewayDeployment := func(cluster *osrmv1alpha1.OSRMCluster) *appsv1.Deployment {
		builder := (&resource.OSRMResourceBuilder{
			Instance: cluster,
			Scheme:   scheme,
		}).GatewayDeployment(cluster.Spec.Profiles)

		obj, err := builder.Build()
		Expect(err).NotTo(HaveOccurred())
		Expect(builder.Update(obj, nil)).To(Succeed())
		return obj.(*appsv1.Deployment)
	}

	Context("Update", func() {
		It("Should use default resources when gateway is not set", func() {
			deployment := buildGatewayDeployment(instance.DeepCopy())
			resources := deployment.Spec.Template.Spec.Containers[0].Resources

			Expect(resources.Requests).To(HaveLen(2))
			Expect(resources.Requests.Memory().String()).To(Equal("500Mi"))
			Expect(resources.Requests.Cpu().String()).To(Equal("500m"))
			Expect(resources.Limits).To(BeEmpty())
		})

		It("Should use default resources when gateway resources are not set", func() {
			cluster := instance.DeepCopy()
			cluster.Spec.Gateway = &osrmv1alpha1.GatewaySpec{}

			deployment := buildGatewayDeployment(cluster)
			resources := deployment.Spec.Template.Spec.Containers[0].Resources

			Expect(resources.Requests.Memory().String()).To(Equal("500Mi"))
			Expect(resources.Requests.Cpu().String()).To(Equal("500m"))
			Expect(resources.Limits).To(BeEmpty())
		})

		It("Should use custom resources when gateway resources are set", func() {
			cluster := instance.DeepCopy()
			cluster.Spec.Gateway = &osrmv1alpha1.GatewaySpec{
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: k8sresource.MustParse("64Mi"),
						corev1.ResourceCPU:    k8sresource.MustParse("50m"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: k8sresource.MustParse("128Mi"),
					},
				},
			}

			deployment := buildGatewayDeployment(cluster)
			resources := deployment.Spec.Template.Spec.Containers[0].Resources

			Expect(resources.Requests).To(HaveLen(2))
			Expect(resources.Requests.Memory().String()).To(Equal("64Mi"))
			Expect(resources.Requests.Cpu().String()).To(Equal("50m"))
			Expect(resources.Limits).To(HaveLen(1))
			Expect(resources.Limits.Memory().String()).To(Equal("128Mi"))
		})
	})
})
