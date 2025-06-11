package controller

import (
	"context"
	"fmt"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sort"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	schedulingapiv1 "github.com/lorenzorottigni/k8s-cj-scheduler/api/v1"
	"github.com/lorenzorottigni/k8s-cj-scheduler/internal/cronjobbuilder"
)

// SchedulerReconciler reconciles a Scheduler object
type SchedulerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SchedulerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var scheduler schedulingapiv1.Scheduler
	if err := r.Get(ctx, req.NamespacedName, &scheduler); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Scheduler resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Scheduler")
		return ctrl.Result{}, err
	}

	// --- 1. Store the original status to compare later if an update is needed
	originalStatus := scheduler.Status.DeepCopy()
	// Initialize status conditions if they are empty
	if originalStatus.Conditions == nil {
		originalStatus.Conditions = []metav1.Condition{}
	}

	// --- 2. Perform reconciliation of CronJobs (create/update/delete) ---
	desiredCronJobsMap := map[string]struct{}{}
	var reconcileErrors []error // Collect errors during CronJob reconciliation
	var latestScheduleTime *metav1.Time

	for _, schedule := range scheduler.Spec.Schedules {
		cronJob := cronjobbuilder.BuildCronJob(&scheduler, schedule)

		if err := ctrl.SetControllerReference(&scheduler, cronJob, r.Scheme); err != nil {
			log.Error(err, "Failed to set owner reference for CronJob", "name", cronJob.Name)
			reconcileErrors = append(reconcileErrors, err)
			continue // Continue to next schedule, try to reconcile others
		}

		desiredCronJobsMap[cronJob.Name] = struct{}{}

		var existing batchv1.CronJob
		err := r.Get(ctx, types.NamespacedName{Name: cronJob.Name, Namespace: cronJob.Namespace}, &existing)
		if err != nil && apierrors.IsNotFound(err) {
			log.Info("Creating CronJob", "name", cronJob.Name)
			if err := r.Create(ctx, cronJob); err != nil {
				log.Error(err, "Failed to create CronJob", "name", cronJob.Name)
				reconcileErrors = append(reconcileErrors, err)
			}
		} else if err != nil {
			log.Error(err, "Failed to get CronJob", "name", cronJob.Name)
			reconcileErrors = append(reconcileErrors, err)
		} else {
			// Update existing CronJob if spec changed
			if !cronJobSpecEqual(&existing.Spec, &cronJob.Spec) {
				existing.Spec = cronJob.Spec // Update spec
				log.Info("Updating CronJob", "name", cronJob.Name)
				if err := r.Update(ctx, &existing); err != nil {
					log.Error(err, "Failed to update CronJob", "name", cronJob.Name)
					reconcileErrors = append(reconcileErrors, err)
				}
			}

			// Update latestScheduleTime
			if existing.Status.LastScheduleTime != nil {
				if latestScheduleTime == nil || existing.Status.LastScheduleTime.After(latestScheduleTime.Time) {
					latestScheduleTime = existing.Status.LastScheduleTime
				}
			}
		}
	}

	// Cleanup old CronJobs that are no longer desired
	if err := r.cleanupCronJobs(ctx, &scheduler, desiredCronJobsMap); err != nil {
		log.Error(err, "Failed to cleanup old CronJobs")
		reconcileErrors = append(reconcileErrors, err)
	}

	// --- 3. Update Status Fields ---
	newStatus := scheduler.Status // Reference to the actual status in scheduler object

	// Set ObservedGeneration
	newStatus.ObservedGeneration = scheduler.Generation

	// Set LastScheduleTime
	newStatus.LastScheduleTime = latestScheduleTime

	// Set Active Jobs
	var activeJobRefs []corev1.ObjectReference
	var activeJobs batchv1.JobList
	// List Jobs owned by this Scheduler
	if err := r.List(ctx, &activeJobs, client.InNamespace(scheduler.Namespace), client.MatchingLabels{"scheduler": scheduler.Name}); err != nil {
		log.Error(err, "Failed to list active Jobs for status update")
		reconcileErrors = append(reconcileErrors, err)
	} else {
		for _, job := range activeJobs.Items {
			// Only consider truly active jobs (not completed or failed)
			if job.Status.Succeeded == 0 && job.Status.Failed == 0 { // Check if job is still considered active
				activeJobRefs = append(activeJobRefs, corev1.ObjectReference{
					Kind:       job.Kind,
					Namespace:  job.Namespace,
					Name:       job.Name,
					UID:        job.UID,
					APIVersion: job.APIVersion,
				})
			}
		}
		// Sort active jobs for consistent ordering in status (important for DeepEqual)
		sort.Slice(activeJobRefs, func(i, j int) bool {
			return activeJobRefs[i].Name < activeJobRefs[j].Name
		})
		newStatus.Active = activeJobRefs
	}

	readyCondition := metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  "ReconcileSuccess",
		Message: "All schedules successfully reconciled and CronJobs are up-to-date.",
	}

	if len(reconcileErrors) > 0 {
		readyCondition.Status = metav1.ConditionFalse
		readyCondition.Reason = "ReconcileError"
		readyCondition.Message = fmt.Sprintf("Encountered %d errors during reconciliation: %v", len(reconcileErrors), reconcileErrors[0].Error())
	}

	meta.SetStatusCondition(&newStatus.Conditions, readyCondition)

	// --- 4. Update the Scheduler's Status subresource if it has changed ---
	if !equality.Semantic.DeepEqual(newStatus, originalStatus) {
		log.Info("Updating Scheduler status")
		if err := r.Status().Update(ctx, &scheduler); err != nil {
			log.Error(err, "Failed to update Scheduler status")
			return ctrl.Result{}, err
		}
	}

	// --- 5. Determine reconcile result ---
	if len(reconcileErrors) > 0 {
		// If there were errors, requeue with backoff to retry
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil // Requeue after 30 seconds
	}

	return ctrl.Result{}, nil
}

// cleanupCronJobs remains the same
func (r *SchedulerReconciler) cleanupCronJobs(ctx context.Context, scheduler *schedulingapiv1.Scheduler, desired map[string]struct{}) error {
	log := log.FromContext(ctx)
	var cronJobList batchv1.CronJobList
	if err := r.List(ctx, &cronJobList, client.InNamespace(scheduler.Namespace), client.MatchingLabels{"scheduler": scheduler.Name}); err != nil {
		return fmt.Errorf("failed to list CronJobs for cleanup: %w", err)
	}

	for _, cj := range cronJobList.Items {
		if _, found := desired[cj.Name]; !found {
			log.Info("Deleting old CronJob", "name", cj.Name)
			if err := r.Delete(ctx, &cj); err != nil {
				return fmt.Errorf("failed to delete old CronJob %s: %w", cj.Name, err)
			}
		}
	}
	return nil
}

// cronJobSpecEqual remains the same
func cronJobSpecEqual(a, b *batchv1.CronJobSpec) bool {
	return equality.Semantic.DeepEqual(a, b)
}

// SetupWithManager sets up the controller with the Manager.
func (r *SchedulerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&schedulingapiv1.Scheduler{}).
		Owns(&batchv1.CronJob{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
