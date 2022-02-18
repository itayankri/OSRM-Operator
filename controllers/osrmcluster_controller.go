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
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	osrmv1alpha1 "github.com/itayankri/OSRM-Opeator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

const pauseReconciliationLabel = "ankri.io/pauseReconciliation"

// OSRMClusterReconciler reconciles a OSRMCluster object
type OSRMClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// the rbac rule requires an empty row at the end to render
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=create
// +kubebuilder:rbac:groups="",resources=pods,verbs=update;get;list;watch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;watch;list
// +kubebuilder:rbac:groups=osrm.ankri.io,resources=osrmclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osrm.ankri.io,resources=osrmclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osrm.ankri.io,resources=osrmclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update

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

	instance, err := r.getOSRMCluster(ctx, req.NamespacedName)
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

	logger.Info("Reconciling OSRMCluster")

	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *OSRMClusterReconciler) getOSRMCluster(ctx context.Context, namespacedName types.NamespacedName) (*osrmv1alpha1.OSRMCluster, error) {
	instance := &osrmv1alpha1.OSRMCluster{}
	err := r.Client.Get(ctx, namespacedName, instance)
	return instance, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *OSRMClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&osrmv1alpha1.OSRMCluster{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Complete(r)
}

func (r *OSRMClusterReconciler) prepareForDeletion(ctx context.Context, instance *osrmv1alpha1.OSRMCluster) error {
	return nil
}

func (r *OSRMClusterReconciler) addFinalizerIfNeeded(ctx context.Context, instance *osrmv1alpha1.OSRMCluster) error {
	return nil
}
