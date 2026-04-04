package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	"github.com/itayankri/OSRM-Operator/internal/metadata"
	"github.com/itayankri/OSRM-Operator/internal/status"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// OSRMReconcilable is satisfied by both *osrmv1alpha1.OSRMCluster and *osrmv1alpha1.OSRMInstance.
type OSRMReconcilable interface {
	client.Object
	SetCondition(metav1.Condition)
	SetConditions([]runtime.Object)
	GetObservedGeneration() int64
	SetObservedGeneration(int64)
	GetPhase() osrmv1alpha1.Phase
	SetPhase(osrmv1alpha1.Phase)
	GetPaused() bool
	SetPaused(bool)
}

func listPersistentVolumeClaims(ctx context.Context, c client.Client, namespace, name string) ([]*corev1.PersistentVolumeClaim, error) {
	listOptions := &client.ListOptions{
		Namespace: namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			metadata.NameLabelKey: name,
		}),
	}
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := c.List(ctx, pvcList, listOptions); err != nil {
		return nil, fmt.Errorf("failed to list PVCs: %w", err)
	}
	pvcs := make([]*corev1.PersistentVolumeClaim, 0, len(pvcList.Items))
	for i := range pvcList.Items {
		pvcs = append(pvcs, &pvcList.Items[i])
	}
	return pvcs, nil
}

func listJobs(ctx context.Context, c client.Client, namespace, name string) ([]*batchv1.Job, error) {
	listOptions := &client.ListOptions{
		Namespace: namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			metadata.NameLabelKey: name,
		}),
	}
	jobList := &batchv1.JobList{}
	if err := c.List(ctx, jobList, listOptions); err != nil {
		return nil, fmt.Errorf("failed to list Jobs: %w", err)
	}
	jobs := make([]*batchv1.Job, 0, len(jobList.Items))
	for i := range jobList.Items {
		jobs = append(jobs, &jobList.Items[i])
	}
	return jobs, nil
}

func computeFutureMapGeneration(pvcs []*corev1.PersistentVolumeClaim) (string, error) {
	highest := 1
	for _, pvc := range pvcs {
		gen := getResourceNameSuffix(pvc.Name)
		n, err := strconv.Atoi(gen)
		if err != nil {
			return "", fmt.Errorf("failed to convert map generation %s to integer: %w", gen, err)
		}
		if n > highest {
			highest = n
		}
	}
	return strconv.Itoa(highest), nil
}

func setReconciliationResult(ctx context.Context, c client.Client, obj OSRMReconcilable, conditionStatus metav1.ConditionStatus, reason, msg string) {
	obj.SetCondition(metav1.Condition{
		Type:    status.ConditionReconciliationSuccess,
		Status:  conditionStatus,
		Reason:  reason,
		Message: msg,
		LastTransitionTime: metav1.Time{
			Time: time.Now(),
		},
	})
	if err := c.Status().Update(ctx, obj); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "Failed to update status",
			"namespace", obj.GetNamespace(),
			"name", obj.GetName())
	}
}

func refreshStatusConditions(ctx context.Context, log logr.Logger, c client.Client, obj OSRMReconcilable, childResources []runtime.Object) (time.Duration, error) {
	obj.SetConditions(childResources)
	if err := c.Status().Update(ctx, obj); err != nil {
		if errors.IsConflict(err) {
			log.Info("failed to update status because of conflict; requeueing...")
			return 2 * time.Second, nil
		}
		return 0, err
	}
	return 0, nil
}

func updateResource(ctx context.Context, c client.Client, obj OSRMReconcilable) error {
	if err := c.Update(ctx, obj); err != nil {
		return err
	}
	obj.SetObservedGeneration(obj.GetGeneration())
	return c.Status().Update(ctx, obj)
}

func initializeResource(ctx context.Context, c client.Client, obj OSRMReconcilable, finalizerName string) error {
	controllerutil.AddFinalizer(obj, finalizerName)
	return updateResource(ctx, c, obj)
}

func cleanupResource(ctx context.Context, c client.Client, obj OSRMReconcilable, finalizerName, conditionMessage string) error {
	if controllerutil.ContainsFinalizer(obj, finalizerName) {
		obj.SetObservedGeneration(obj.GetGeneration())
		obj.SetCondition(metav1.Condition{
			Type:    status.ConditionAvailable,
			Status:  metav1.ConditionFalse,
			Reason:  "Cleanup",
			Message: conditionMessage,
		})
		obj.SetPhase(osrmv1alpha1.PhaseDeleting)

		if err := c.Status().Update(ctx, obj); err != nil {
			return err
		}

		controllerutil.RemoveFinalizer(obj, finalizerName)

		if err := c.Update(ctx, obj); err != nil {
			return err
		}
	}

	obj.SetObservedGeneration(obj.GetGeneration())
	err := c.Status().Update(ctx, obj)
	if errors.IsConflict(err) || errors.IsNotFound(err) {
		return nil
	}
	return err
}

func applyLastSpecAnnotation(ctx context.Context, c client.Client, obj client.Object, annotationKey string, spec interface{}) error {
	newSpecJSON, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("failed to marshal spec: %w", err)
	}
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[annotationKey] = string(newSpecJSON)
	obj.SetAnnotations(annotations)
	return c.Update(ctx, obj)
}
