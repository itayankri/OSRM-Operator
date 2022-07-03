package controllers_test

import (
	"context"
	"fmt"
	"time"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ClusterCreationTimeout = 60 * time.Second
	ClusterDeletionTimeout = 15 * time.Second
)

var _ = Describe("OSRMClusterController", func() {
	var (
		cluster          *osrmv1alpha1.OSRMCluster
		defaultNamespace = "default"
		defaultImage     = "osrm/osrm-backend"
	)

	Context("Single profile", func() {
		BeforeEach(func() {
			cluster = getOSRMClusterCR()

			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, k8sClient)
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				return apierrors.IsNotFound(err)
			}, 5).Should(BeTrue())
		})

		It("should be able to create a single-profile OSRM cluster", func() {
			By("populating the image spec with the default image", func() {
				fetchedCluster := &osrmv1alpha1.OSRMCluster{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "rabbitmq-one", Namespace: defaultNamespace}, fetchedCluster)).To(Succeed())
				Expect(fetchedCluster.Spec.Image).To(Equal(defaultImage))
			})
		})
	})

	Context("Multiple profiles", func() {

	})

	Context("Annotations on the instance", func() {

	})

	Context("Persistence configurations", func() {

	})

	Context("Custom resource updates", func() {

	})

	Context("Recreate child resources after deletion", func() {

	})

	Context("Pause reconciliation", func() {

	})
})

func getOSRMClusterCR() *osrmv1alpha1.OSRMCluster {
	storage := resource.MustParse("100Mi")
	return &osrmv1alpha1.OSRMCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: osrmv1alpha1.OSRMClusterSpec{
			PBFURL: "",
			Profiles: osrmv1alpha1.ProfilesSpec{
				{
					Name:         "car",
					EndpointName: "driving",
					MinReplicas:  &[]int32{1}[0],
					MaxReplicas:  &[]int32{10}[0],
				},
				{
					Name:         "bicycle",
					EndpointName: "cycling",
					MinReplicas:  &[]int32{0}[0],
					MaxReplicas:  &[]int32{3}[0],
				},
			},
			Service: osrmv1alpha1.ServiceSpec{
				ExposingServices: []string{"route", "table", "match"},
			},
			Persistence: osrmv1alpha1.PersistenceSpec{
				Storage:          &storage,
				StorageClassName: "Standard",
			},
		},
	}
}

func waitForClusterCreation(ctx context.Context, rabbitmqCluster *osrmv1alpha1.OSRMCluster, client runtimeClient.Client) {
	EventuallyWithOffset(1, func() string {
		osrmClusterCreated := osrmv1alpha1.OSRMCluster{}
		if err := client.Get(
			ctx,
			types.NamespacedName{
				Name:      rabbitmqCluster.Name,
				Namespace: rabbitmqCluster.Namespace,
			},
			&osrmClusterCreated,
		); err != nil {
			return fmt.Sprintf("%v+", err)
		}

		if len(osrmClusterCreated.Status.Conditions) == 0 {
			return "not ready"
		}

		return "ready"

	}, ClusterCreationTimeout, 1*time.Second).Should(Equal("ready"))
}
