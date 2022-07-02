package controllers_test

import (
	"context"
	"fmt"
	"time"

	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
	)

	Context("Single profile", func() {
		BeforeEach(func() {
			cluster = &osrmv1alpha1.OSRMCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-one",
					Namespace: defaultNamespace,
				},
			}

			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, k8sClient)
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
