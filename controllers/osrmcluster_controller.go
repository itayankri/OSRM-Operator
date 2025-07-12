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
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/metadata"
	"github.com/itayankri/OSRM-Operator/internal/resource"
	"github.com/itayankri/OSRM-Operator/internal/status"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientretry "k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const finalizerName = "osrmcluster.itayankri/finalizer"
const mapGenerationAnnotation = "osrmcluster.itayankri/map-generation"
const lastAppliedSpecAnnotation = "osrmcluster.itayankri/last-applied-spec"

// OSRMClusterReconciler reconciles a OSRMCluster object
type OSRMClusterReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	log              logr.Logger
	DefaultOSRMImage string
}

func NewOSRMClusterReconciler(client client.Client, scheme *runtime.Scheme) *OSRMClusterReconciler {
	return &OSRMClusterReconciler{
		Client: client,
		Scheme: scheme,
		log:    ctrl.Log.WithName("controller").WithName("OSRM"),
	}
}

func isInitialized(instance *osrmv1alpha1.OSRMCluster) bool {
	return controllerutil.ContainsFinalizer(instance, finalizerName)
}

func isBeingDeleted(object metav1.Object) bool {
	return !object.GetDeletionTimestamp().IsZero()
}

func isPaused(object metav1.Object) bool {
	if object.GetAnnotations() == nil {
		return false
	}
	pausedStr, ok := object.GetAnnotations()[osrmv1alpha1.OperatorPausedAnnotation]
	if !ok {
		return false
	}
	paused, err := strconv.ParseBool(pausedStr)
	if err != nil {
		return false
	}
	return paused
}

func getMapGeneration(childResources []runtime.Object) (string, error) {
	for _, resource := range childResources {
		if pvc, ok := resource.(*corev1.PersistentVolumeClaim); ok {
			if generation, ok := pvc.ObjectMeta.Annotations[mapGenerationAnnotation]; ok {
				return generation, nil
			}
		}
	}

	return "", fmt.Errorf("no PersistentVolumeClaim with map generation found")
}

// the rbac rule requires an empty row at the end to render
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=create
// +kubebuilder:rbac:groups="",resources=pods,verbs=update;get;list;watch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;deletecollection
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="batch",resources=jobs,verbs=get;list;watch;create;update;deletecollection
// +kubebuilder:rbac:groups="batch",resources=cronjobs,verbs=get;list;watch;create;update;deletecollection
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;create;update;deletecollection
// +kubebuilder:rbac:groups="autoscaling",resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;deletecollection
// +kubebuilder:rbac:groups="networking.k8s.io",resources=ingresses,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;watch;list
// +kubebuilder:rbac:groups="policy",resources=poddisruptionbudgets,verbs=get;list;watch;create;update;deletecollection
// +kubebuilder:rbac:groups=osrm.itayankri,resources=osrmclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osrm.itayankri,resources=osrmclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osrm.itayankri,resources=osrmclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;deletecollection
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
	logger := r.log.WithValues("OSRM", req.NamespacedName)
	logger.Info("Starting reconciliation")

	instance, err := r.getOSRMCluster(ctx, req.NamespacedName)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Instance not found")

			// No need to requeue if the resource no longer exists
			return reconcile.Result{}, nil
		}

		logger.Error(err, "Failed to fetch OSRMCluster")

		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	childResources, err := r.getChildResources(ctx, instance)
	if err != nil {
		logger.Error(err, "Failed to fetch child resources", instance.Namespace, instance.Name)
		r.setReconciliationSuccess(ctx, instance, metav1.ConditionFalse, "FailedToFetchChildResources", err.Error())
		return ctrl.Result{}, err
	}

	if requeueAfter, err := r.updateStatusConditions(ctx, instance, childResources); err != nil || requeueAfter > 0 {
		return ctrl.Result{RequeueAfter: requeueAfter}, err
	}

	var mapGeneration string
	if !isInitialized(instance) {
		mapGeneration = "1"
		err := r.initialize(ctx, instance)
		// No need to requeue here, because
		// the update will trigger reconciliation again
		logger.Info("OSRMCLuster initialized")
		return ctrl.Result{}, err
	} else {
		mapGeneration, err = getMapGeneration(childResources)
	}

	if isBeingDeleted(instance) {
		err := r.cleanup(ctx, instance)
		if err != nil {
			logger.Error(err, "Cleanup failed for rerouce: %v/%v", instance.Namespace, instance.Name)
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	if isPaused(instance) {
		if instance.Status.Paused {
			logger.Info("OSRM operator is paused on resource: %v/%v", instance.Namespace, instance.Name)
			return ctrl.Result{}, nil
		}
		logger.Info(fmt.Sprintf("Pausing OSRM operator on resource: %v/%v", instance.Namespace, instance.Name))
		instance.Status.Paused = true
		err := r.updateOSRMClusterResource(ctx, instance)
		return ctrl.Result{}, err
	}

	rawInstanceSpec, err := json.Marshal(instance.Spec)
	if err != nil {
		logger.Error(err, "Failed to marshal OSRMCluster spec")
	}

	logger.Info("Reconciling OSRMCluster", "spec", string(rawInstanceSpec))

	resourceBuilder := resource.OSRMResourceBuilder{
		Instance:      instance,
		Scheme:        r.Scheme,
		MapGeneration: mapGeneration,
	}

	builders := resourceBuilder.ResourceBuilders()

	for _, builder := range builders {
		if builder.ShouldDeploy(childResources) {
			resource, err := builder.Build()
			if err != nil {
				logger.Error(err, "Failed to build resource %v for OSRMCluster %v/%v", builder, instance.Namespace, instance.Name)
				r.setReconciliationSuccess(ctx, instance, metav1.ConditionFalse, "FailedToBuildChildResource", err.Error())
				return ctrl.Result{}, err
			}

			var operationResult controllerutil.OperationResult
			err = clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
				var apiError error
				operationResult, apiError = controllerutil.CreateOrUpdate(ctx, r.Client, resource, func() error {
					return builder.Update(resource, childResources)
				})
				return apiError
			})
			r.logOperationResult(logger, instance, resource, operationResult, err)
			if err != nil {
				r.setReconciliationSuccess(ctx, instance, metav1.ConditionFalse, "Error", err.Error())
				return ctrl.Result{}, err
			}
		}
	}

	err = r.setLastAppliedSpecAnnotation(ctx, instance)
	if err != nil {
		logger.Error(err, "Garbage collection failed for OSRMCluster %v/%v", instance.Namespace, instance.Name)
		return ctrl.Result{RequeueAfter: time.Second * 10}, err
	}

	err = r.garbageCollection(ctx, instance)
	if err != nil {
		logger.Error(err, "Garbage collection failed for OSRMCluster %v/%v", instance.Namespace, instance.Name)
		return ctrl.Result{RequeueAfter: time.Second * 10}, err
	}

	r.setReconciliationSuccess(ctx, instance, metav1.ConditionTrue, "Success", "Reconciliation completed")
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
func (r *OSRMClusterReconciler) logOperationResult(
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

func (r *OSRMClusterReconciler) setLastAppliedSpecAnnotation(
	ctx context.Context,
	instance *osrmv1alpha1.OSRMCluster,
) error {
	newSpecjson, err := json.Marshal(instance.Spec)
	if err != nil {
		return fmt.Errorf("failed to marshal OSRMCluster spec: %w", err)
	}
	if instance.Annotations == nil {
		instance.Annotations = make(map[string]string)
	}
	instance.Annotations[lastAppliedSpecAnnotation] = string(newSpecjson)
	return r.Client.Update(ctx, instance)
}

func (r *OSRMClusterReconciler) setReconciliationSuccess(
	ctx context.Context,
	osrmCluster *osrmv1alpha1.OSRMCluster,
	conditionStatus metav1.ConditionStatus,
	reason, msg string,
) {
	osrmCluster.Status.SetCondition(metav1.Condition{
		Type:    status.ConditionReconciliationSuccess,
		Status:  conditionStatus,
		Reason:  reason,
		Message: msg,
		LastTransitionTime: metav1.Time{
			Time: time.Now(),
		},
	})
	if err := r.Status().Update(ctx, osrmCluster); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "Failed to update Custom Resource status",
			"namespace", osrmCluster.Namespace,
			"name", osrmCluster.Name)
	}
}

func (r *OSRMClusterReconciler) initialize(ctx context.Context, instance *osrmv1alpha1.OSRMCluster) error {
	controllerutil.AddFinalizer(instance, finalizerName)
	return r.updateOSRMClusterResource(ctx, instance)
}

func (r *OSRMClusterReconciler) updateOSRMClusterResource(ctx context.Context, instance *osrmv1alpha1.OSRMCluster) error {
	err := r.Client.Update(ctx, instance)
	if err != nil {
		return err
	}

	instance.Status.ObservedGeneration = instance.Generation
	return r.Client.Status().Update(ctx, instance)
}

func (r *OSRMClusterReconciler) updateStatusConditions(
	ctx context.Context,
	instance *osrmv1alpha1.OSRMCluster,
	childResources []runtime.Object,
) (time.Duration, error) {
	instance.Status.SetConditions(childResources)
	err := r.Client.Status().Update(ctx, instance)
	if err != nil {
		if errors.IsConflict(err) {
			r.log.Info("failed to update status because of conflict; requeueing...")
			return 2 * time.Second, nil
		}
		return 0, err
	}
	return 0, nil
}

func (r *OSRMClusterReconciler) getChildResources(ctx context.Context, instance *osrmv1alpha1.OSRMCluster) ([]runtime.Object, error) {
	children := []runtime.Object{}

	gatewayDeployment := &appsv1.Deployment{}
	if err := r.Client.Get(ctx, types.NamespacedName{
		Name:      instance.ChildResourceName(resource.GatewaySuffix, resource.DeploymentSuffix),
		Namespace: instance.Namespace,
	}, gatewayDeployment); err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
	} else {
		children = append(children, gatewayDeployment)
	}

	gatewayService := &corev1.Service{}
	if err := r.Client.Get(ctx, types.NamespacedName{
		Name:      instance.ChildResourceName(resource.GatewaySuffix, resource.ServiceSuffix),
		Namespace: instance.Namespace,
	}, gatewayService); err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
	} else {
		children = append(children, gatewayService)
	}

	gatewayConfigMap := &corev1.ConfigMap{}
	if err := r.Client.Get(ctx, types.NamespacedName{
		Name:      instance.ChildResourceName(resource.GatewaySuffix, resource.ConfigMapSuffix),
		Namespace: instance.Namespace,
	}, gatewayConfigMap); err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
	} else {
		children = append(children, gatewayConfigMap)
	}

	for _, profileSpec := range instance.Spec.Profiles {
		pvc := &corev1.PersistentVolumeClaim{}
		if err := r.Client.Get(ctx, types.NamespacedName{
			Name:      instance.ChildResourceName(profileSpec.Name, resource.PersistentVolumeClaimSuffix),
			Namespace: instance.Namespace,
		}, pvc); err != nil {
			if !errors.IsNotFound(err) {
				return nil, err
			}
		} else {
			children = append(children, pvc)
		}

		job := &batchv1.Job{}
		if err := r.Client.Get(ctx, types.NamespacedName{
			Name:      instance.ChildResourceName(profileSpec.Name, resource.JobSuffix),
			Namespace: instance.Namespace,
		}, job); err != nil {
			if !errors.IsNotFound(err) {
				return nil, err
			}
		} else {
			children = append(children, job)
		}

		cronJob := &batchv1.CronJob{}
		if err := r.Client.Get(ctx, types.NamespacedName{
			Name:      instance.ChildResourceName(profileSpec.Name, resource.CronJobSuffix),
			Namespace: instance.Namespace,
		}, cronJob); err != nil {
			if !errors.IsNotFound(err) {
				return nil, err
			}
		} else {
			children = append(children, cronJob)
		}

		deployment := &appsv1.Deployment{}
		if err := r.Client.Get(ctx, types.NamespacedName{
			Name:      instance.ChildResourceName(profileSpec.Name, resource.DeploymentSuffix),
			Namespace: instance.Namespace,
		}, deployment); err != nil {
			if !errors.IsNotFound(err) {
				return nil, err
			}
		} else {
			children = append(children, deployment)
		}

		hpa := &autoscalingv1.HorizontalPodAutoscaler{}
		if err := r.Client.Get(ctx, types.NamespacedName{
			Name:      instance.ChildResourceName(profileSpec.Name, resource.HorizontalPodAutoscalerSuffix),
			Namespace: instance.Namespace,
		}, hpa); err != nil {
			if !errors.IsNotFound(err) {
				return nil, err
			}
		} else {
			children = append(children, hpa)
		}

		pdb := &policyv1.PodDisruptionBudget{}
		if err := r.Client.Get(ctx, types.NamespacedName{
			Name:      instance.ChildResourceName(profileSpec.Name, resource.PodDisruptionBudgetSuffix),
			Namespace: instance.Namespace,
		}, pdb); err != nil {
			if !errors.IsNotFound(err) {
				return nil, err
			}
		} else {
			children = append(children, pdb)
		}

		service := &corev1.Service{}
		if err := r.Client.Get(ctx, types.NamespacedName{
			Name:      instance.ChildResourceName(profileSpec.Name, resource.ServiceSuffix),
			Namespace: instance.Namespace,
		}, service); err != nil {
			if !errors.IsNotFound(err) {
				return nil, err
			}
		} else {
			children = append(children, service)
		}
	}

	return children, nil
}

func (r *OSRMClusterReconciler) garbageCollection(ctx context.Context, instance *osrmv1alpha1.OSRMCluster) error {
	labelSelector := fmt.Sprintf(
		"%s=%s,%s=%s,%s,%s notin (%d)",
		metadata.NameLabelKey,
		instance.Name,
		metadata.ComponentLabelKey,
		metadata.ComponentLabelProfile,
		metadata.GenerationLabelKey,
		metadata.GenerationLabelKey,
		instance.ObjectMeta.Generation,
	)
	propagationPolicy := metav1.DeletePropagationBackground

	err := r.Client.DeleteAllOf(ctx, &batchv1.CronJob{}, &client.DeleteAllOfOptions{
		ListOptions: client.ListOptions{
			Namespace: instance.Namespace,
			Raw:       &metav1.ListOptions{LabelSelector: labelSelector},
		},
		DeleteOptions: client.DeleteOptions{PropagationPolicy: &propagationPolicy},
	})
	if err != nil {
		return err
	}

	err = r.Client.DeleteAllOf(ctx, &batchv1.Job{}, &client.DeleteAllOfOptions{
		ListOptions: client.ListOptions{
			Namespace: instance.Namespace,
			Raw:       &metav1.ListOptions{LabelSelector: labelSelector},
		},
		DeleteOptions: client.DeleteOptions{PropagationPolicy: &propagationPolicy},
	})
	if err != nil {
		return err
	}

	err = r.Client.DeleteAllOf(ctx, &autoscalingv1.HorizontalPodAutoscaler{}, &client.DeleteAllOfOptions{
		ListOptions: client.ListOptions{
			Namespace: instance.Namespace,
			Raw:       &metav1.ListOptions{LabelSelector: labelSelector},
		},
	})
	if err != nil {
		return err
	}

	err = r.Client.DeleteAllOf(ctx, &policyv1.PodDisruptionBudget{}, &client.DeleteAllOfOptions{
		ListOptions: client.ListOptions{
			Namespace: instance.Namespace,
			Raw:       &metav1.ListOptions{LabelSelector: labelSelector},
		},
	})
	if err != nil {
		return err
	}

	err = r.Client.DeleteAllOf(ctx, &corev1.Service{}, &client.DeleteAllOfOptions{
		ListOptions: client.ListOptions{
			Namespace: instance.Namespace,
			Raw:       &metav1.ListOptions{LabelSelector: labelSelector},
		},
	})
	if err != nil {
		return err
	}

	err = r.Client.DeleteAllOf(ctx, &appsv1.Deployment{}, &client.DeleteAllOfOptions{
		ListOptions: client.ListOptions{
			Namespace: instance.Namespace,
			Raw:       &metav1.ListOptions{LabelSelector: labelSelector},
		},
	})
	if err != nil {
		return err
	}

	err = r.Client.DeleteAllOf(ctx, &corev1.PersistentVolumeClaim{}, &client.DeleteAllOfOptions{
		ListOptions: client.ListOptions{
			Namespace: instance.Namespace,
			Raw:       &metav1.ListOptions{LabelSelector: labelSelector},
		},
	})

	return err
}

func (r *OSRMClusterReconciler) cleanup(ctx context.Context, instance *osrmv1alpha1.OSRMCluster) error {
	if controllerutil.ContainsFinalizer(instance, finalizerName) {
		instance.Status.ObservedGeneration = instance.Generation
		instance.Status.SetCondition(metav1.Condition{
			Type:    status.ConditionAvailable,
			Status:  metav1.ConditionFalse,
			Reason:  "Cleanup",
			Message: "Deleting OSRMCluster resources",
		})

		err := r.Client.Status().Update(ctx, instance)
		if err != nil {
			return err
		}

		controllerutil.RemoveFinalizer(instance, finalizerName)

		err = r.Client.Update(ctx, instance)
		if err != nil {
			return err
		}
	}

	instance.Status.ObservedGeneration = instance.Generation
	err := r.Client.Status().Update(ctx, instance)
	if errors.IsConflict(err) || errors.IsNotFound(err) {
		// These errors are ignored. They can happen if the CR was removed
		// before the status update call is executed.
		return nil
	}
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *OSRMClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&osrmv1alpha1.OSRMCluster{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&autoscalingv1.HorizontalPodAutoscaler{}).
		Complete(r)
}
