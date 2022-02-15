/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	osrmv1alpha1 "github.com/itayankri/OSRM-Opeator/api/v1alpha1"
)

// OSRMClusterReconciler reconciles a OSRMCluster object
type OSRMClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=osrm.ankri.io,resources=osrmclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=osrm.ankri.io,resources=osrmclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=osrm.ankri.io,resources=osrmclusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OSRMCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *OSRMClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling OSRMCluster")

	// TODO(user): your logic here
	instance := &osrmv1alpha1.OSRMCluster{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// No need to requeue if the resource no longer exists
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting")
		return ctrl.Result{}, r.prepareForDeletion(ctx, instance)
	}

	// Ensure the resource have a deletion marker
	if err := r.addFinalizerIfNeeded(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OSRMClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&osrmv1alpha1.OSRMCluster{}).
		Complete(r)
}

func (r *OSRMClusterReconciler) prepareForDeletion(ctx context.Context, instance *osrmv1alpha1.OSRMCluster) error {
	return nil
}

func (r *OSRMClusterReconciler) addFinalizerIfNeeded(ctx context.Context, instance *osrmv1alpha1.OSRMCluster) error {
	return nil
}
