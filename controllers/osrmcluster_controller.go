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
	"strings"
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
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientretry "k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const finalizerName = "osrmcluster.itayankri/finalizer"
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

func getResourceNameSuffix(name string) string {
	tokens := strings.Split(name, "-")
	return tokens[len(tokens)-1]
}

// the rbac rule requires an empty row at the end to render
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=create
// +kubebuilder:rbac:groups="",resources=pods,verbs=update;get;list;watch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="batch",resources=jobs,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="batch",resources=cronjobs,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="autoscaling",resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="networking.k8s.io",resources=ingresses,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;watch;list
// +kubebuilder:rbac:groups="policy",resources=poddisruptionbudgets,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=osrm.itayankri,resources=osrmclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osrm.itayankri,resources=osrmclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osrm.itayankri,resources=osrmclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;delete
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

	if !isInitialized(instance) {
		err := r.initialize(ctx, instance)
		// No need to requeue here, because
		// the update will trigger reconciliation again
		logger.Info("OSRMCluster initialized")
		return ctrl.Result{}, err
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

	oldSpecJSON, annotationFound := instance.GetAnnotations()[lastAppliedSpecAnnotation]
	oldSpec := &osrmv1alpha1.OSRMClusterSpec{}
	if annotationFound {
		if err := json.Unmarshal([]byte(oldSpecJSON), oldSpec); err != nil {
			logger.Error(err, "Failed to marshal last applied spec of OSRMCluster %v/%v", instance.Namespace, instance.Name)
			return ctrl.Result{}, err
		}
	}

	pvcs, _ := r.getPersistentVolumeClaims(ctx, instance)
	jobs, _ := r.getJobs(ctx, instance)
	profilesDeployments, _ := r.getProfilesDeployments(ctx, instance)
	activeMapGeneration, err := r.GetActiveMapGeneration(profilesDeployments)
	logger.Info("Number of resources", "pvcs", len(pvcs), "jobs", len(jobs), "profilesDeployments", len(profilesDeployments), "activeMapGeneration", activeMapGeneration)
	if err != nil {
		logger.Error(err, "Failed to get active map generation for OSRMCluster")
		return ctrl.Result{}, err
	}

	futureMapGeneration, err := r.GetFutureMapGeneration(pvcs)
	if err != nil {
		logger.Error(err, "Failed to get future map generation for OSRMCluster")
		return ctrl.Result{}, err
	}

	phase := r.DeterminePhase(
		instance,
		oldSpec,
		activeMapGeneration,
		futureMapGeneration,
		pvcs,
		jobs,
		profilesDeployments,
	)

	logger.Info("Reconciling OSRMCluster", "phase", phase)

	resourceBuilder := resource.NewOSRMResourceBuilder(
		instance,
		r.Scheme,
		activeMapGeneration,
		futureMapGeneration,
	)

	if instance.Status.Phase != phase {
		instance.Status.Phase = phase
		if err := r.Client.Status().Update(ctx, instance); err != nil {
			logger.Error(err, "Failed to update phase for OSRMCluster %v/%v", instance.Namespace, instance.Name)
			return ctrl.Result{}, err
		}
	}

	builders := resourceBuilder.ResourceBuildersForPhase(phase)

	for _, builder := range builders {
		resource, err := builder.Build()
		if err != nil {
			logger.Error(err, "Failed to build resource %v for OSRMCluster %v/%v", builder, instance.Namespace, instance.Name)
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

	err = r.setLastAppliedSpecAnnotation(ctx, instance)
	if err != nil {
		logger.Error(err, "Failed to apply last spec annotation for OSRMCluster %v/%v", instance.Namespace, instance.Name)
		return ctrl.Result{RequeueAfter: time.Second * 10}, err
	}

	err = r.GarbageCollection(ctx, instance)
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

func IsMapBuildingInProgress(
	instance *osrmv1alpha1.OSRMCluster,
	pvcs []*corev1.PersistentVolumeClaim,
	jobs []*batchv1.Job,
	mapGeneration string,
	logger logr.Logger,
) bool {

	allVolumesBound := true
	for _, profile := range instance.Spec.Profiles {
		pvcFound := false
		for _, pvc := range pvcs {
			if pvc.Name == instance.ChildResourceName(profile.Name, mapGeneration) {
				pvcFound = true
				if !status.IsPersistentVolumeClaimBound(pvc) {
					allVolumesBound = false
					break
				}
			}
		}

		logger.Info("IsMapBuildingInProgress pvcFound", "lookedFor", instance.ChildResourceName(profile.Name, mapGeneration), "found", pvcFound)
		if !pvcFound {
			allVolumesBound = false
			break
		}
	}

	allMapsBuilt := true
	for _, profile := range instance.Spec.Profiles {
		jobFound := false
		for _, job := range jobs {
			if job.Name == instance.ChildResourceName(profile.Name, mapGeneration) {
				jobFound = true
				if !status.IsJobCompleted(job) {
					allMapsBuilt = false
					break
				}
			}
		}

		logger.Info("IsMapBuildingInProgress jobFound", "lookedFor", instance.ChildResourceName(profile.Name, mapGeneration), "found", jobFound)
		if !jobFound {
			allMapsBuilt = false
			break
		}
	}

	logger.Info("IsMapBuildingInProgress results", "allVolumesBound", allVolumesBound, "allMapsBuilt", allMapsBuilt)
	return !allVolumesBound || !allMapsBuilt
}

// DeterminePhase is exported for testing
func (r *OSRMClusterReconciler) DeterminePhase(
	instance *osrmv1alpha1.OSRMCluster,
	oldSpec *osrmv1alpha1.OSRMClusterSpec,
	activeMapGeneration string,
	futureMapGeneration string,
	pvcs []*corev1.PersistentVolumeClaim,
	jobs []*batchv1.Job,
	profilesDeployments []*appsv1.Deployment,
) osrmv1alpha1.Phase {
	if len(instance.Spec.Profiles) == 0 {
		return osrmv1alpha1.PhaseEmpty
	}

	if IsMapBuildingInProgress(instance, pvcs, jobs, activeMapGeneration, r.log) {
		return osrmv1alpha1.PhaseBuildingMap
	}

	if instance.Spec.PBFURL != oldSpec.PBFURL || IsMapBuildingInProgress(instance, pvcs, jobs, futureMapGeneration, r.log) {
		return osrmv1alpha1.PhaseUpdatingMap
	}

	if activeMapGeneration != futureMapGeneration {
		return osrmv1alpha1.PhaseRedepoloyingWorkers
	}

	allWorkersDeployed := true
	for _, profile := range instance.Spec.Profiles {
		deploymentName := instance.ChildResourceName(profile.Name, resource.DeploymentSuffix)

		deploymentReady := false
		for _, deployment := range profilesDeployments {
			if deployment.Name == deploymentName {
				if deployment.Status.ReadyReplicas > 0 && deployment.Status.ReadyReplicas == deployment.Status.Replicas {
					deploymentReady = true
					break
				}
			}
		}

		if !deploymentReady {
			allWorkersDeployed = false
			break
		}
	}

	activeMapGenerationInteger, _ := strconv.Atoi(activeMapGeneration)

	if !allWorkersDeployed {
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

func (r *OSRMClusterReconciler) GetActiveMapGeneration(profilesDeployments []*appsv1.Deployment) (string, error) {
	activeMapGeneration := "1"
	if len(profilesDeployments) > 0 {
		suffix := getResourceNameSuffix(profilesDeployments[0].Spec.Template.Spec.Volumes[0].VolumeSource.PersistentVolumeClaim.ClaimName)
		_, err := strconv.Atoi(suffix)
		if err != nil {
			return "", fmt.Errorf("failed to convert map generation %s to integer: %w", suffix, err)
		}
		activeMapGeneration = suffix
	} else {
		r.log.Info("No profile deployments found, using default active map generation 1")
	}

	return activeMapGeneration, nil
}

// GetFutureMapGeneration is exported for testing
func (r *OSRMClusterReconciler) GetFutureMapGeneration(pvcs []*corev1.PersistentVolumeClaim) (string, error) {
	higestMapGeneration := 1
	for _, pvc := range pvcs {
		mapGeneration := getResourceNameSuffix(pvc.Name)
		mapGenerationInteger, err := strconv.Atoi(mapGeneration)
		if err != nil {
			return "", fmt.Errorf("failed to convert map generation %s to integer: %w", mapGeneration, err)
		}

		if mapGenerationInteger > higestMapGeneration {
			higestMapGeneration = mapGenerationInteger
		}
	}
	return strconv.Itoa(higestMapGeneration), nil
}

func (r *OSRMClusterReconciler) getProfilesDeployments(
	ctx context.Context,
	instance *osrmv1alpha1.OSRMCluster,
) ([]*appsv1.Deployment, error) {
	deployments := make([]*appsv1.Deployment, 0)
	for _, profileSpec := range instance.Spec.Profiles {
		deployment := &appsv1.Deployment{}
		if err := r.Client.Get(ctx, types.NamespacedName{
			Name:      instance.ChildResourceName(profileSpec.Name, resource.DeploymentSuffix),
			Namespace: instance.Namespace,
		}, deployment); err != nil {
			if !errors.IsNotFound(err) {
				return nil, err
			}
		} else {
			deployments = append(deployments, deployment)
		}
	}
	return deployments, nil
}

func (r *OSRMClusterReconciler) getPersistentVolumeClaims(
	ctx context.Context,
	instance *osrmv1alpha1.OSRMCluster,
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
	for _, pvc := range pvcList.Items {
		pvcs = append(pvcs, &pvc)
	}
	return pvcs, nil
}

func (r *OSRMClusterReconciler) getJobs(ctx context.Context,
	instance *osrmv1alpha1.OSRMCluster,
) ([]*batchv1.Job, error) {
	listOptions := &client.ListOptions{
		Namespace: instance.Namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			metadata.NameLabelKey: instance.Name,
		}),
	}

	jobList := &batchv1.JobList{}
	if err := r.Client.List(ctx, jobList, listOptions); err != nil {
		return nil, fmt.Errorf("failed to list PVCs: %w", err)
	}

	jobs := make([]*batchv1.Job, 0)
	for _, job := range jobList.Items {
		jobs = append(jobs, &job)
	}
	return jobs, nil
}

func (r *OSRMClusterReconciler) getChildResources(
	ctx context.Context,
	instance *osrmv1alpha1.OSRMCluster,
) ([]runtime.Object, error) {
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

func (r *OSRMClusterReconciler) GarbageCollection(ctx context.Context, instance *osrmv1alpha1.OSRMCluster) error {
	propagationPolicy := metav1.DeletePropagationBackground

	expectedResources := make(map[string]bool)
	for _, profile := range instance.Spec.Profiles {
		expectedResources[instance.ChildResourceName(profile.Name, resource.DeploymentSuffix)] = true
		expectedResources[instance.ChildResourceName(profile.Name, resource.ServiceSuffix)] = true
		expectedResources[instance.ChildResourceName(profile.Name, resource.HorizontalPodAutoscalerSuffix)] = true
		expectedResources[instance.ChildResourceName(profile.Name, resource.PodDisruptionBudgetSuffix)] = true

		if profile.SpeedUpdates != nil {
			expectedResources[instance.ChildResourceName(profile.Name, resource.CronJobSuffix)] = true
		}
	}

	if len(instance.Spec.Profiles) > 0 {
		expectedResources[instance.ChildResourceName(resource.GatewaySuffix, resource.DeploymentSuffix)] = true
		expectedResources[instance.ChildResourceName(resource.GatewaySuffix, resource.ServiceSuffix)] = true
		expectedResources[instance.ChildResourceName(resource.GatewaySuffix, resource.ConfigMapSuffix)] = true
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
	for _, cronJob := range cronJobList.Items {
		if !expectedResources[cronJob.Name] {
			err := r.Client.Delete(ctx, &cronJob, &client.DeleteOptions{PropagationPolicy: &propagationPolicy})
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete CronJob %s: %w", cronJob.Name, err)
			}
		}
	}

	deploymentList := &appsv1.DeploymentList{}
	if err := r.Client.List(ctx, deploymentList, listOptions); err != nil {
		return fmt.Errorf("failed to list Deployments: %w", err)
	}
	for _, deployment := range deploymentList.Items {
		if !expectedResources[deployment.Name] {
			r.log.Info(fmt.Sprintf("Deleting Deployment %s while reconciling instance %s", deployment.Name, instance.Name))
			err := r.Client.Delete(ctx, &deployment, &client.DeleteOptions{PropagationPolicy: &propagationPolicy})
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete Deployment %s: %w", deployment.Name, err)
			}
		}
	}

	hpaList := &autoscalingv1.HorizontalPodAutoscalerList{}
	if err := r.Client.List(ctx, hpaList, listOptions); err != nil {
		return fmt.Errorf("failed to list HPAs: %w", err)
	}
	for _, hpa := range hpaList.Items {
		if !expectedResources[hpa.Name] {
			r.log.Info(fmt.Sprintf("Deleting HorizontalPodAutoscalerList %s while reconciling instance %s", hpa.Name, instance.Name))
			err := r.Client.Delete(ctx, &hpa, &client.DeleteOptions{PropagationPolicy: &propagationPolicy})
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete HPA %s: %w", hpa.Name, err)
			}
		}
	}

	pdbList := &policyv1.PodDisruptionBudgetList{}
	if err := r.Client.List(ctx, pdbList, listOptions); err != nil {
		return fmt.Errorf("failed to list PDBs: %w", err)
	}
	for _, pdb := range pdbList.Items {
		if !expectedResources[pdb.Name] {
			r.log.Info(fmt.Sprintf("Deleting PodDisruptionBudgetList %s while reconciling instance %s", pdb.Name, instance.Name))
			err := r.Client.Delete(ctx, &pdb, &client.DeleteOptions{PropagationPolicy: &propagationPolicy})
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete PDB %s: %w", pdb.Name, err)
			}
		}
	}

	serviceList := &corev1.ServiceList{}
	if err := r.Client.List(ctx, serviceList, listOptions); err != nil {
		return fmt.Errorf("failed to list Services: %w", err)
	}
	for _, service := range serviceList.Items {
		if !expectedResources[service.Name] {
			r.log.Info(fmt.Sprintf("Deleting Service %s while reconciling instance %s", service.Name, instance.Name))
			err := r.Client.Delete(ctx, &service, &client.DeleteOptions{PropagationPolicy: &propagationPolicy})
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete Service %s: %w", service.Name, err)
			}
		}
	}

	configMapList := &corev1.ConfigMapList{}
	if err := r.Client.List(ctx, configMapList, listOptions); err != nil {
		return fmt.Errorf("failed to list ConfigMaps: %w", err)
	}
	for _, configMap := range configMapList.Items {
		if !expectedResources[configMap.Name] {
			err := r.Client.Delete(ctx, &configMap, &client.DeleteOptions{PropagationPolicy: &propagationPolicy})
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete ConfigMap %s: %w", configMap.Name, err)
			}
		}
	}

	expectedPVCs := make(map[string]bool)
	expectedJobs := make(map[string]bool)

	pvcs, err := r.getPersistentVolumeClaims(ctx, instance)
	if err != nil {
		return fmt.Errorf("failed to get PVCs for garbage collection: %w", err)
	}

	jobs, err := r.getJobs(ctx, instance)
	if err != nil {
		return fmt.Errorf("failed to get Jobs for garbage collection: %w", err)
	}

	profilesDeployments, err := r.getProfilesDeployments(ctx, instance)
	if err != nil {
		return fmt.Errorf("failed to get profiles deployments for garbage collection: %w", err)
	}

	activeMapGeneration, err := r.GetActiveMapGeneration(profilesDeployments)
	if err != nil {
		return fmt.Errorf("failed to get active map generation: %w", err)
	}

	futureMapGeneration, err := r.GetFutureMapGeneration(pvcs)
	if err != nil {
		return fmt.Errorf("failed to get future map generation: %w", err)
	}

	for _, profile := range instance.Spec.Profiles {
		// Always keep the active generation (what deployments are currently using)
		expectedPVCs[instance.ChildResourceName(profile.Name, activeMapGeneration)] = true
		expectedJobs[instance.ChildResourceName(profile.Name, activeMapGeneration)] = true

		// During blue-green deployment, also keep the future generation
		if activeMapGeneration != futureMapGeneration {
			// Only keep future generation during active deployment phases
			phase := instance.Status.Phase
			if phase == osrmv1alpha1.PhaseUpdatingMap ||
				phase == osrmv1alpha1.PhaseRedepoloyingWorkers ||
				phase == osrmv1alpha1.PhaseDeployingWorkers {
				expectedPVCs[instance.ChildResourceName(profile.Name, futureMapGeneration)] = true
				expectedJobs[instance.ChildResourceName(profile.Name, futureMapGeneration)] = true
			}
			// In PhaseWorkersRedeployed, only keep active generation to clean up old future generation
		}
	}

	for _, pvc := range pvcs {
		if !expectedPVCs[pvc.Name] {
			r.log.Info(fmt.Sprintf("Deleting PersistentVolumeClaim %s while reconciling instance %s", pvc.Name, instance.Name))
			err := r.Client.Delete(ctx, pvc, &client.DeleteOptions{PropagationPolicy: &propagationPolicy})
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete PVC %s: %w", pvc.Name, err)
			}
		}
	}

	for _, job := range jobs {
		if !expectedJobs[job.Name] {
			r.log.Info(fmt.Sprintf("Deleting Job %s while reconciling instance %s", job.Name, instance.Name))
			err := r.Client.Delete(ctx, job, &client.DeleteOptions{PropagationPolicy: &propagationPolicy})
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete Job %s: %w", job.Name, err)
			}
		}
	}

	return nil
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
		instance.Status.Phase = osrmv1alpha1.PhaseDeleting

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
