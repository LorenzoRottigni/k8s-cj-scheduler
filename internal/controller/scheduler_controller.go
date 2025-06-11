/*
Copyright 2025.

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

package controller

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	schedulerv1 "github.com/LorenzoRottigni/k8s-scheduler/api/v1"
)

// SchedulerReconciler reconciles a Scheduler object
type SchedulerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=scheduling.deesup.com,resources=schedulers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scheduling.deesup.com,resources=schedulers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=scheduling.deesup.com,resources=schedulers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Scheduler object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
// func (r *SchedulerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
// 	_ = logf.FromContext(ctx)
//
// 	// TODO(user): your logic here
//
// 	return ctrl.Result{}, nil
// }

// SetupWithManager sets up the controller with the Manager.
func (r *SchedulerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&schedulingv1.Scheduler{}).
		Named("scheduler").
		Complete(r)
}

func (r *SchedulerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var scheduler schedulerv1.Scheduler
	if err := r.Get(ctx, req.NamespacedName, &scheduler); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	cronExpr, ok1 := scheduler.Annotations["scheduler.deesup.com/cron"]
	operation, ok2 := scheduler.Annotations["scheduler.deesup.com/operation"]

	if !ok1 || !ok2 {
		log.Info("Missing required annotations 'scheduler.deesup.com/cron' or 'scheduler.deesup.com/operation'")
		// optionally set condition or event here
		return ctrl.Result{}, nil
	}

	cronJobName := scheduler.Name + "-cronjob"

	// Build your CronJob spec with cronExpr and operation
	cronJob := buildCronJob(cronJobName, scheduler.Namespace, cronExpr, operation)

	// Set owner reference so CronJob is deleted if Scheduler is deleted
	if err := ctrl.SetControllerReference(&scheduler, cronJob, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Create or Update CronJob
	var existing batchv1.CronJob
	err := r.Get(ctx, types.NamespacedName{Name: cronJobName, Namespace: scheduler.Namespace}, &existing)
	if err != nil && apierrors.IsNotFound(err) {
		if err := r.Create(ctx, cronJob); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("Created CronJob", "cronJob", cronJobName)
	} else if err == nil {
		// Update logic if needed (e.g., cron schedule changed)
		existing.Spec.Schedule = cronExpr
		existing.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Args = []string{operation}
		if err := r.Update(ctx, &existing); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("Updated CronJob", "cronJob", cronJobName)
	} else {
		return ctrl.Result{}, err
	}

	// Update Scheduler status
	scheduler.Status.CronJobName = cronJobName
	if err := r.Status().Update(ctx, &scheduler); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func buildCronJob(name, namespace, schedule, operation string) *batchv1.CronJob {
	// Build a simple CronJob using batchv1 API
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule: schedule,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyOnFailure,
							Containers: []corev1.Container{
								{
									Name:  "operation",
									Image: "your-operation-image", // set your image
									Args:  []string{operation},
								},
							},
						},
					},
				},
			},
		},
	}
}
