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
	"k8s.io/apimachinery/pkg/labels"
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
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientretry "k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const instanceFinalizerName = "osrminstance.itayankri/finalizer"
const instanceLastAppliedSpecAnnotation = "osrminstance.itayankri/last-applied-spec"

type OSRMInstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	log    logr.Logger
}

func NewOSRMInstanceReconciler(client client.Client, scheme *runtime.Scheme) *OSRMInstanceReconciler {
	return &OSRMInstanceReconciler{
		Client: client,
		Scheme: scheme,
		log:    ctrl.Log.WithName("controller").WithName("OSRMInstance"),
	}
}

func isInstanceInitialized(instance *osrmv1alpha1.OSRMInstance) bool {
	return controllerutil.ContainsFinalizer(instance, instanceFinalizerName)
}

// +kubebuilder:rbac:groups=osrm.itayankri,resources=osrminstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osrm.itayankri,resources=osrminstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osrm.itayankri,resources=osrminstances/finalizers,verbs=update

func (r *OSRMInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.log.WithValues("OSRMInstance", req.NamespacedName)
	logger.Info("Starting reconciliation")

	instance, err := r.getOSRMInstance(ctx, req.NamespacedName)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		logger.Error(err, "Failed to fetch OSRMInstance")
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

	if !isInstanceInitialized(instance) {
		err := r.initialize(ctx, instance)
		logger.Info("OSRMInstance initialized")
		return ctrl.Result{}, err
	}

	if isBeingDeleted(instance) {
		if err := r.cleanup(ctx, instance); err != nil {
			logger.Error(err, "Cleanup failed for resource: %v/%v", instance.Namespace, instance.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if isPaused(instance) {
		if instance.Status.Paused {
			return ctrl.Result{}, nil
		}
		instance.Status.Paused = true
		return ctrl.Result{}, r.updateOSRMInstanceResource(ctx, instance)
	}

	oldSpecJSON, annotationFound := instance.GetAnnotations()[instanceLastAppliedSpecAnnotation]
	oldSpec := &osrmv1alpha1.OSRMInstanceSpec{}
	if annotationFound {
		if err := json.Unmarshal([]byte(oldSpecJSON), oldSpec); err != nil {
			logger.Error(err, "Failed to unmarshal last applied spec of OSRMInstance %v/%v", instance.Namespace, instance.Name)
			return ctrl.Result{}, err
		}
	}

	pvcs, _ := r.getPersistentVolumeClaims(ctx, instance)
	jobs, _ := r.getJobs(ctx, instance)
	deployment, _ := r.getDeployment(ctx, instance)
	activeMapGeneration, err := r.GetActiveMapGeneration(deployment)
	if err != nil {
		logger.Error(err, "Failed to get active map generation for OSRMInstance")
		return ctrl.Result{}, err
	}

	futureMapGeneration, err := r.GetFutureMapGeneration(pvcs)
	if err != nil {
		logger.Error(err, "Failed to get future map generation for OSRMInstance")
		return ctrl.Result{}, err
	}

	phase := r.DeterminePhase(
		instance,
		oldSpec,
		activeMapGeneration,
		futureMapGeneration,
		pvcs,
		jobs,
		deployment,
	)

	logger.Info("Reconciling OSRMInstance")

	resourceBuilder := resource.NewOSRMInstanceResourceBuilder(
		instance,
		r.Scheme,
		activeMapGeneration,
		futureMapGeneration,
	)

	if instance.Status.Phase != phase {
		instance.Status.Phase = phase
		if err := r.Client.Status().Update(ctx, instance); err != nil {
			logger.Error(err, "Failed to update phase for OSRMInstance %v/%v", instance.Namespace, instance.Name)
			return ctrl.Result{}, err
		}
	}

	builders := resourceBuilder.ResourceBuildersForPhase(phase)

	for _, builder := range builders {
		resource, err := builder.Build()
		if err != nil {
			logger.Error(err, "Failed to build resource for OSRMInstance %v/%v", instance.Namespace, instance.Name)
			r.setReconciliationSuccess(ctx, instance, metav1.ConditionFalse, "FailedToBuildChildResource", err.Error())
			return ctrl.Result{}, err
		}

		err = clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			var apiError error
			_, apiError = controllerutil.CreateOrUpdate(ctx, r.Client, resource, func() error {
				return builder.Update(resource, childResources)
			})
			return apiError
		})
		if err != nil {
			r.setReconciliationSuccess(ctx, instance, metav1.ConditionFalse, "Error", err.Error())
			return ctrl.Result{}, err
		}
	}

	if err = r.setLastAppliedSpecAnnotation(ctx, instance); err != nil {
		logger.Error(err, "Failed to apply last spec annotation for OSRMInstance %v/%v", instance.Namespace, instance.Name)
		return ctrl.Result{RequeueAfter: time.Second * 10}, err
	}

	if err = r.GarbageCollection(ctx, instance); err != nil {
		logger.Error(err, "Garbage collection failed for OSRMInstance %v/%v", instance.Namespace, instance.Name)
		return ctrl.Result{RequeueAfter: time.Second * 10}, err
	}

	r.setReconciliationSuccess(ctx, instance, metav1.ConditionTrue, "Success", "Reconciliation completed")
	logger.Info("Finished reconciling")
	return ctrl.Result{}, nil
}

func (r *OSRMInstanceReconciler) getOSRMInstance(ctx context.Context, namespacedName types.NamespacedName) (*osrmv1alpha1.OSRMInstance, error) {
	instance := &osrmv1alpha1.OSRMInstance{}
	err := r.Client.Get(ctx, namespacedName, instance)
	return instance, err
}

func IsMapBuildingInProgressForInstance(
	instance *osrmv1alpha1.OSRMInstance,
	pvcs []*corev1.PersistentVolumeClaim,
	jobs []*batchv1.Job,
	mapGeneration string,
) bool {
	pvcName := instance.ChildResourceName("", mapGeneration)

	pvcBound := false
	for _, pvc := range pvcs {
		if pvc.Name == pvcName {
			pvcBound = status.IsPersistentVolumeClaimBound(pvc)
			break
		}
	}

	jobCompleted := false
	for _, job := range jobs {
		if job.Name == pvcName { // job has the same name as the pvc
			jobCompleted = status.IsJobCompleted(job)
			break
		}
	}

	return !pvcBound || !jobCompleted
}

// DeterminePhase is exported for testing
func (r *OSRMInstanceReconciler) DeterminePhase(
	instance *osrmv1alpha1.OSRMInstance,
	oldSpec *osrmv1alpha1.OSRMInstanceSpec,
	activeMapGeneration string,
	futureMapGeneration string,
	pvcs []*corev1.PersistentVolumeClaim,
	jobs []*batchv1.Job,
	deployment *appsv1.Deployment,
) osrmv1alpha1.Phase {
	if IsMapBuildingInProgressForInstance(instance, pvcs, jobs, activeMapGeneration) {
		return osrmv1alpha1.PhaseBuildingMap
	}

	if instance.Spec.PBFURL != oldSpec.PBFURL || IsMapBuildingInProgressForInstance(instance, pvcs, jobs, futureMapGeneration) {
		return osrmv1alpha1.PhaseUpdatingMap
	}

	if activeMapGeneration != futureMapGeneration {
		return osrmv1alpha1.PhaseRedepoloyingWorkers
	}

	deploymentReady := deployment != nil &&
		deployment.Status.ReadyReplicas > 0 &&
		deployment.Status.ReadyReplicas == deployment.Status.Replicas

	activeMapGenerationInteger, _ := strconv.Atoi(activeMapGeneration)

	if !deploymentReady {
		if activeMapGenerationInteger > 1 {
			return osrmv1alpha1.PhaseRedepoloyingWorkers
		}
		return osrmv1alpha1.PhaseDeployingWorkers
	}

	if activeMapGenerationInteger > 1 {
		return osrmv1alpha1.PhaseWorkersRedeployed
	}
	return osrmv1alpha1.PhaseWorkersDeployed
}

func (r *OSRMInstanceReconciler) setLastAppliedSpecAnnotation(
	ctx context.Context,
	instance *osrmv1alpha1.OSRMInstance,
) error {
	newSpecJSON, err := json.Marshal(instance.Spec)
	if err != nil {
		return fmt.Errorf("failed to marshal OSRMInstance spec: %w", err)
	}
	if instance.Annotations == nil {
		instance.Annotations = make(map[string]string)
	}
	instance.Annotations[instanceLastAppliedSpecAnnotation] = string(newSpecJSON)
	return r.Client.Update(ctx, instance)
}

func (r *OSRMInstanceReconciler) setReconciliationSuccess(
	ctx context.Context,
	instance *osrmv1alpha1.OSRMInstance,
	conditionStatus metav1.ConditionStatus,
	reason, msg string,
) {
	instance.Status.SetCondition(metav1.Condition{
		Type:    status.ConditionReconciliationSuccess,
		Status:  conditionStatus,
		Reason:  reason,
		Message: msg,
		LastTransitionTime: metav1.Time{
			Time: time.Now(),
		},
	})
	if err := r.Status().Update(ctx, instance); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "Failed to update OSRMInstance status",
			"namespace", instance.Namespace,
			"name", instance.Name)
	}
}

func (r *OSRMInstanceReconciler) initialize(ctx context.Context, instance *osrmv1alpha1.OSRMInstance) error {
	controllerutil.AddFinalizer(instance, instanceFinalizerName)
	return r.updateOSRMInstanceResource(ctx, instance)
}

func (r *OSRMInstanceReconciler) updateOSRMInstanceResource(ctx context.Context, instance *osrmv1alpha1.OSRMInstance) error {
	if err := r.Client.Update(ctx, instance); err != nil {
		return err
	}
	instance.Status.ObservedGeneration = instance.Generation
	return r.Client.Status().Update(ctx, instance)
}

func (r *OSRMInstanceReconciler) updateStatusConditions(
	ctx context.Context,
	instance *osrmv1alpha1.OSRMInstance,
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

func (r *OSRMInstanceReconciler) GetActiveMapGeneration(deployment *appsv1.Deployment) (string, error) {
	if deployment == nil {
		r.log.Info("No deployment found, using default active map generation 1")
		return "1", nil
	}
	if len(deployment.Spec.Template.Spec.Volumes) == 0 {
		return "1", nil
	}
	pvcClaim := deployment.Spec.Template.Spec.Volumes[0].VolumeSource.PersistentVolumeClaim
	if pvcClaim == nil {
		return "1", nil
	}
	suffix := getResourceNameSuffix(pvcClaim.ClaimName)
	if _, err := strconv.Atoi(suffix); err != nil {
		return "", fmt.Errorf("failed to convert map generation %s to integer: %w", suffix, err)
	}
	return suffix, nil
}

// GetFutureMapGeneration is exported for testing
func (r *OSRMInstanceReconciler) GetFutureMapGeneration(pvcs []*corev1.PersistentVolumeClaim) (string, error) {
	highestMapGeneration := 1
	for _, pvc := range pvcs {
		mapGeneration := getResourceNameSuffix(pvc.Name)
		mapGenerationInteger, err := strconv.Atoi(mapGeneration)
		if err != nil {
			return "", fmt.Errorf("failed to convert map generation %s to integer: %w", mapGeneration, err)
		}
		if mapGenerationInteger > highestMapGeneration {
			highestMapGeneration = mapGenerationInteger
		}
	}
	return strconv.Itoa(highestMapGeneration), nil
}

func (r *OSRMInstanceReconciler) getDeployment(
	ctx context.Context,
	instance *osrmv1alpha1.OSRMInstance,
) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	if err := r.Client.Get(ctx, types.NamespacedName{
		Name:      instance.ChildResourceName("", resource.DeploymentSuffix),
		Namespace: instance.Namespace,
	}, deployment); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return deployment, nil
}

func (r *OSRMInstanceReconciler) getPersistentVolumeClaims(
	ctx context.Context,
	instance *osrmv1alpha1.OSRMInstance,
) ([]*corev1.PersistentVolumeClaim, error) {
	listOptions := &client.ListOptions{
		Namespace: instance.Namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			metadata.NameLabelKey: instance.Name,
		}),
	}

	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := r.Client.List(ctx, pvcList, listOptions); err != nil {
		return nil, fmt.Errorf("failed to list PVCs: %w", err)
	}

	pvcs := make([]*corev1.PersistentVolumeClaim, 0)
	for i := range pvcList.Items {
		pvcs = append(pvcs, &pvcList.Items[i])
	}
	return pvcs, nil
}

func (r *OSRMInstanceReconciler) getJobs(
	ctx context.Context,
	instance *osrmv1alpha1.OSRMInstance,
) ([]*batchv1.Job, error) {
	listOptions := &client.ListOptions{
		Namespace: instance.Namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			metadata.NameLabelKey: instance.Name,
		}),
	}

	jobList := &batchv1.JobList{}
	if err := r.Client.List(ctx, jobList, listOptions); err != nil {
		return nil, fmt.Errorf("failed to list Jobs: %w", err)
	}

	jobs := make([]*batchv1.Job, 0)
	for i := range jobList.Items {
		jobs = append(jobs, &jobList.Items[i])
	}
	return jobs, nil
}

func (r *OSRMInstanceReconciler) getChildResources(
	ctx context.Context,
	instance *osrmv1alpha1.OSRMInstance,
) ([]runtime.Object, error) {
	children := []runtime.Object{}

	deployment := &appsv1.Deployment{}
	if err := r.Client.Get(ctx, types.NamespacedName{
		Name:      instance.ChildResourceName("", resource.DeploymentSuffix),
		Namespace: instance.Namespace,
	}, deployment); err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
	} else {
		children = append(children, deployment)
	}

	svc := &corev1.Service{}
	if err := r.Client.Get(ctx, types.NamespacedName{
		Name:      instance.ChildResourceName("", resource.ServiceSuffix),
		Namespace: instance.Namespace,
	}, svc); err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
	} else {
		children = append(children, svc)
	}

	hpa := &autoscalingv1.HorizontalPodAutoscaler{}
	if err := r.Client.Get(ctx, types.NamespacedName{
		Name:      instance.ChildResourceName("", resource.HorizontalPodAutoscalerSuffix),
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
		Name:      instance.ChildResourceName("", resource.PodDisruptionBudgetSuffix),
		Namespace: instance.Namespace,
	}, pdb); err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
	} else {
		children = append(children, pdb)
	}

	if instance.Spec.SpeedUpdates != nil {
		cronJob := &batchv1.CronJob{}
		if err := r.Client.Get(ctx, types.NamespacedName{
			Name:      instance.ChildResourceName("", resource.CronJobSuffix),
			Namespace: instance.Namespace,
		}, cronJob); err != nil {
			if !errors.IsNotFound(err) {
				return nil, err
			}
		} else {
			children = append(children, cronJob)
		}
	}

	return children, nil
}

func (r *OSRMInstanceReconciler) GarbageCollection(ctx context.Context, instance *osrmv1alpha1.OSRMInstance) error {
	propagationPolicy := metav1.DeletePropagationBackground

	expectedResources := map[string]bool{
		instance.ChildResourceName("", resource.DeploymentSuffix):              true,
		instance.ChildResourceName("", resource.ServiceSuffix):                 true,
		instance.ChildResourceName("", resource.HorizontalPodAutoscalerSuffix): true,
		instance.ChildResourceName("", resource.PodDisruptionBudgetSuffix):     true,
	}
	if instance.Spec.SpeedUpdates != nil {
		expectedResources[instance.ChildResourceName("", resource.CronJobSuffix)] = true
	}

	listOptions := &client.ListOptions{
		Namespace: instance.Namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			metadata.NameLabelKey: instance.Name,
		}),
	}

	cronJobList := &batchv1.CronJobList{}
	if err := r.Client.List(ctx, cronJobList, listOptions); err != nil {
		return fmt.Errorf("failed to list CronJobs: %w", err)
	}
	for i := range cronJobList.Items {
		cj := &cronJobList.Items[i]
		if !expectedResources[cj.Name] {
			if err := r.Client.Delete(ctx, cj, &client.DeleteOptions{PropagationPolicy: &propagationPolicy}); err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete CronJob %s: %w", cj.Name, err)
			}
		}
	}

	deploymentList := &appsv1.DeploymentList{}
	if err := r.Client.List(ctx, deploymentList, listOptions); err != nil {
		return fmt.Errorf("failed to list Deployments: %w", err)
	}
	for i := range deploymentList.Items {
		d := &deploymentList.Items[i]
		if !expectedResources[d.Name] {
			if err := r.Client.Delete(ctx, d, &client.DeleteOptions{PropagationPolicy: &propagationPolicy}); err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete Deployment %s: %w", d.Name, err)
			}
		}
	}

	hpaList := &autoscalingv1.HorizontalPodAutoscalerList{}
	if err := r.Client.List(ctx, hpaList, listOptions); err != nil {
		return fmt.Errorf("failed to list HPAs: %w", err)
	}
	for i := range hpaList.Items {
		h := &hpaList.Items[i]
		if !expectedResources[h.Name] {
			if err := r.Client.Delete(ctx, h, &client.DeleteOptions{PropagationPolicy: &propagationPolicy}); err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete HPA %s: %w", h.Name, err)
			}
		}
	}

	pdbList := &policyv1.PodDisruptionBudgetList{}
	if err := r.Client.List(ctx, pdbList, listOptions); err != nil {
		return fmt.Errorf("failed to list PDBs: %w", err)
	}
	for i := range pdbList.Items {
		p := &pdbList.Items[i]
		if !expectedResources[p.Name] {
			if err := r.Client.Delete(ctx, p, &client.DeleteOptions{PropagationPolicy: &propagationPolicy}); err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete PDB %s: %w", p.Name, err)
			}
		}
	}

	serviceList := &corev1.ServiceList{}
	if err := r.Client.List(ctx, serviceList, listOptions); err != nil {
		return fmt.Errorf("failed to list Services: %w", err)
	}
	for i := range serviceList.Items {
		s := &serviceList.Items[i]
		if !expectedResources[s.Name] {
			if err := r.Client.Delete(ctx, s, &client.DeleteOptions{PropagationPolicy: &propagationPolicy}); err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete Service %s: %w", s.Name, err)
			}
		}
	}

	// PVC and Job GC: keep active generation and (during update phases) future generation
	pvcs, err := r.getPersistentVolumeClaims(ctx, instance)
	if err != nil {
		return fmt.Errorf("failed to get PVCs for garbage collection: %w", err)
	}
	jobs, err := r.getJobs(ctx, instance)
	if err != nil {
		return fmt.Errorf("failed to get Jobs for garbage collection: %w", err)
	}
	deployment, err := r.getDeployment(ctx, instance)
	if err != nil {
		return fmt.Errorf("failed to get deployment for garbage collection: %w", err)
	}
	activeMapGeneration, err := r.GetActiveMapGeneration(deployment)
	if err != nil {
		return fmt.Errorf("failed to get active map generation: %w", err)
	}
	futureMapGeneration, err := r.GetFutureMapGeneration(pvcs)
	if err != nil {
		return fmt.Errorf("failed to get future map generation: %w", err)
	}

	expectedPVCs := map[string]bool{
		instance.ChildResourceName("", activeMapGeneration): true,
	}
	expectedJobs := map[string]bool{
		instance.ChildResourceName("", activeMapGeneration): true,
	}

	if activeMapGeneration != futureMapGeneration {
		phase := instance.Status.Phase
		if phase == osrmv1alpha1.PhaseUpdatingMap ||
			phase == osrmv1alpha1.PhaseRedepoloyingWorkers ||
			phase == osrmv1alpha1.PhaseDeployingWorkers {
			expectedPVCs[instance.ChildResourceName("", futureMapGeneration)] = true
			expectedJobs[instance.ChildResourceName("", futureMapGeneration)] = true
		}
	}

	for _, pvc := range pvcs {
		if !expectedPVCs[pvc.Name] {
			r.log.Info(fmt.Sprintf("Deleting PVC %s while reconciling instance %s", pvc.Name, instance.Name))
			if err := r.Client.Delete(ctx, pvc, &client.DeleteOptions{PropagationPolicy: &propagationPolicy}); err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete PVC %s: %w", pvc.Name, err)
			}
		}
	}

	for _, job := range jobs {
		if !expectedJobs[job.Name] {
			r.log.Info(fmt.Sprintf("Deleting Job %s while reconciling instance %s", job.Name, instance.Name))
			if err := r.Client.Delete(ctx, job, &client.DeleteOptions{PropagationPolicy: &propagationPolicy}); err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete Job %s: %w", job.Name, err)
			}
		}
	}

	return nil
}

func (r *OSRMInstanceReconciler) cleanup(ctx context.Context, instance *osrmv1alpha1.OSRMInstance) error {
	if controllerutil.ContainsFinalizer(instance, instanceFinalizerName) {
		instance.Status.ObservedGeneration = instance.Generation
		instance.Status.SetCondition(metav1.Condition{
			Type:    status.ConditionAvailable,
			Status:  metav1.ConditionFalse,
			Reason:  "Cleanup",
			Message: "Deleting OSRMInstance resources",
		})
		instance.Status.Phase = osrmv1alpha1.PhaseDeleting

		if err := r.Client.Status().Update(ctx, instance); err != nil {
			return err
		}

		controllerutil.RemoveFinalizer(instance, instanceFinalizerName)

		if err := r.Client.Update(ctx, instance); err != nil {
			return err
		}
	}

	instance.Status.ObservedGeneration = instance.Generation
	err := r.Client.Status().Update(ctx, instance)
	if errors.IsConflict(err) || errors.IsNotFound(err) {
		return nil
	}
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *OSRMInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&osrmv1alpha1.OSRMInstance{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Owns(&corev1.Service{}).
		Owns(&autoscalingv1.HorizontalPodAutoscaler{}).
		Complete(r)
}
