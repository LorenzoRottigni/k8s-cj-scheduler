// controllers/scheduler_controller.go
package controllers

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	schedulingapiv1 "github.com/lorenzorottigni/k8s-scheduler/api/v1"
)

// SchedulerReconciler reconciles a Scheduler object
type SchedulerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=your.domain,resources=schedulers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=your.domain,resources=schedulers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete

func (r *SchedulerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var scheduler schedulingapiv1.Scheduler
	if err := r.Get(ctx, req.NamespacedName, &scheduler); err != nil {
		if apierrors.IsNotFound(err) {
			// Scheduler resource not found. Return and don't requeue.
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch Scheduler")
		return ctrl.Result{}, err
	}

	// Track the names of CronJobs that should exist for cleanup later
	desiredCronJobs := map[string]struct{}{}

	for _, schedule := range scheduler.Spec.Schedules {
		cronJob := r.buildCronJob(&scheduler, schedule)

		desiredCronJobs[cronJob.Name] = struct{}{}

		var existing batchv1.CronJob
		err := r.Get(ctx, types.NamespacedName{Name: cronJob.Name, Namespace: cronJob.Namespace}, &existing)
		if err != nil && apierrors.IsNotFound(err) {
			log.Info("Creating CronJob", "name", cronJob.Name)
			if err := r.Create(ctx, cronJob); err != nil {
				log.Error(err, "Failed to create CronJob", "name", cronJob.Name)
				return ctrl.Result{}, err
			}
		} else if err != nil {
			log.Error(err, "Failed to get CronJob", "name", cronJob.Name)
			return ctrl.Result{}, err
		} else {
			// Check if spec changed, update if necessary
			if !cronJobSpecEqual(&existing.Spec, &cronJob.Spec) {
				existing.Spec = cronJob.Spec
				log.Info("Updating CronJob", "name", cronJob.Name)
				if err := r.Update(ctx, &existing); err != nil {
					log.Error(err, "Failed to update CronJob", "name", cronJob.Name)
					return ctrl.Result{}, err
				}
			}
		}
	}

	// Optional: Cleanup old CronJobs that no longer exist in Spec.Schedules
	if err := r.cleanupCronJobs(ctx, &scheduler, desiredCronJobs); err != nil {
		log.Error(err, "Failed to cleanup old CronJobs")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// buildCronJob builds a CronJob from the Scheduler and Schedule specs
func (r *SchedulerReconciler) buildCronJob(scheduler *schedulingapiv1.Scheduler, schedule schedulingapiv1.Schedule) *batchv1.CronJob {
	name := scheduler.Name + "-" + schedule.Name
	labels := map[string]string{
		"app":       "scheduler-controller",
		"scheduler": scheduler.Name,
		"schedule":  schedule.Name,
	}

	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: scheduler.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(scheduler, schedulingapiv1.GroupVersion.WithKind("Scheduler")),
			},
		},
		Spec: batchv1.CronJobSpec{
			Schedule: schedule.CronExpression,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyOnFailure,
							Containers: []corev1.Container{
								{
									Name:  "job",
									Image: schedule.Image,
									Args:  schedule.Params,
								},
							},
						},
					},
				},
			},
		},
	}
}

// cleanupCronJobs deletes any CronJobs owned by this Scheduler but no longer in Spec.Schedules
func (r *SchedulerReconciler) cleanupCronJobs(ctx context.Context, scheduler *schedulingapiv1.Scheduler, desired map[string]struct{}) error {
	var cronJobList batchv1.CronJobList
	if err := r.List(ctx, &cronJobList, client.InNamespace(scheduler.Namespace), client.MatchingLabels{"scheduler": scheduler.Name}); err != nil {
		return err
	}

	for _, cj := range cronJobList.Items {
		if _, found := desired[cj.Name]; !found {
			if err := r.Delete(ctx, &cj); err != nil {
				return err
			}
		}
	}
	return nil
}

// cronJobSpecEqual compares two CronJob specs for equality, ignoring fields that don't affect scheduling
func cronJobSpecEqual(a, b *batchv1.CronJobSpec) bool {
	// Basic check on schedule string and container specs, can be improved for thoroughness
	if a.Schedule != b.Schedule {
		return false
	}
	if len(a.JobTemplate.Spec.Template.Spec.Containers) != len(b.JobTemplate.Spec.Template.Spec.Containers) {
		return false
	}

	for i := range a.JobTemplate.Spec.Template.Spec.Containers {
		ac := a.JobTemplate.Spec.Template.Spec.Containers[i]
		bc := b.JobTemplate.Spec.Template.Spec.Containers[i]
		if ac.Image != bc.Image {
			return false
		}
		if len(ac.Args) != len(bc.Args) {
			return false
		}
		for j := range ac.Args {
			if ac.Args[j] != bc.Args[j] {
				return false
			}
		}
	}
	return true
}

// SetupWithManager sets up the controller with the Manager.
func (r *SchedulerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&schedulingapiv1.Scheduler{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}
