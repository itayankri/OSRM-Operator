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
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/resource"
	"github.com/itayankri/OSRM-Operator/internal/status"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientretry "k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const pauseReconciliationLabel = "ankri.io/pauseReconciliation"

// OSRMClusterReconciler reconciles a OSRMCluster object
type OSRMClusterReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	DefaultOSRMImage string
}

// the rbac rule requires an empty row at the end to render
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=create
// +kubebuilder:rbac:groups="",resources=pods,verbs=update;get;list;watch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="autoscaling",resources=horizontalpodautoscalers,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="networking.k8s.io",resources=ingresses,verbs=get;list;watch;create;update
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

	rawInstanceSpec, err := json.Marshal(instance.Spec)
	if err != nil {
		logger.Error(err, "Failed to marshal cluster spec")
	}

	logger.Info("Reconciling OSRMCluster", "spec", string(rawInstanceSpec))

	resourceBuilder := resource.OSRMResourceBuilder{
		Instance: instance,
		Scheme:   r.Scheme,
	}

	builders := resourceBuilder.ResourceBuilders()

	for _, builder := range builders {
		resource, err := builder.Build()
		if err != nil {
			return ctrl.Result{}, err
		}

		var operationResult controllerutil.OperationResult
		err = clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			var apiError error
			operationResult, apiError = controllerutil.CreateOrUpdate(ctx, r.Client, resource, func() error {
				return builder.Update(resource)
			})
			return apiError
		})
		r.logAndRecordOperationResult(logger, instance, resource, operationResult, err)
		if err != nil {
			r.setReconcileSuccess(ctx, instance, corev1.ConditionFalse, "Error", err.Error())
			return ctrl.Result{}, err
		}
	}

	r.setReconcileSuccess(ctx, instance, corev1.ConditionTrue, "Success", "Reconciliation completed")
	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *OSRMClusterReconciler) getOSRMCluster(ctx context.Context, namespacedName types.NamespacedName) (*osrmv1alpha1.OSRMCluster, error) {
	instance := &osrmv1alpha1.OSRMCluster{}
	err := r.Client.Get(ctx, namespacedName, instance)
	return instance, err
}

// logAndRecordOperationResult - helper function to log and record events with message and error
// it logs and records 'updated' and 'created' OperationResult, and ignores OperationResult 'unchanged'
func (r *OSRMClusterReconciler) logAndRecordOperationResult(
	logger logr.Logger,
	ro runtime.Object,
	resource runtime.Object,
	operationResult controllerutil.OperationResult,
	err error,
) {
	if operationResult == controllerutil.OperationResultNone && err == nil {
		return
	}

	var operation string
	if operationResult == controllerutil.OperationResultCreated {
		operation = "create"
	}
	if operationResult == controllerutil.OperationResultUpdated {
		operation = "update"
	}

	if err == nil {
		msg := fmt.Sprintf("%sd resource %s of Type %T", operation, resource.(metav1.Object).GetName(), resource.(metav1.Object))
		logger.Info(msg)
	}

	if err != nil {
		msg := fmt.Sprintf("failed to %s resource %s of Type %T", operation, resource.(metav1.Object).GetName(), resource.(metav1.Object))
		logger.Error(err, msg)
	}
}

func (r *OSRMClusterReconciler) setReconcileSuccess(
	ctx context.Context,
	rabbitmqCluster *osrmv1alpha1.OSRMCluster,
	condition corev1.ConditionStatus,
	reason, msg string,
) {
	rabbitmqCluster.Status.SetCondition(status.ReconcileSuccess, condition, reason, msg)
	if writerErr := r.Status().Update(ctx, rabbitmqCluster); writerErr != nil {
		ctrl.LoggerFrom(ctx).Error(writerErr, "Failed to update Custom Resource status",
			"namespace", rabbitmqCluster.Namespace,
			"name", rabbitmqCluster.Name)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *OSRMClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&osrmv1alpha1.OSRMCluster{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&autoscalingv1.HorizontalPodAutoscaler{}).
		Complete(r)
}

func (r *OSRMClusterReconciler) prepareForDeletion(ctx context.Context, instance *osrmv1alpha1.OSRMCluster) error {
	return nil
}

func (r *OSRMClusterReconciler) addFinalizerIfNeeded(ctx context.Context, instance *osrmv1alpha1.OSRMCluster) error {
	return nil
}
