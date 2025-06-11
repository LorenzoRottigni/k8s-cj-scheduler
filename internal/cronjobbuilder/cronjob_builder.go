package cronjobbuilder

import (
	schedulingapiv1 "github.com/lorenzorottigni/k8s-cj-scheduler/api/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildCronJob creates a Kubernetes CronJob object from a Scheduler custom resource.
func BuildCronJob(scheduler *schedulingapiv1.Scheduler, schedule schedulingapiv1.Schedule) *batchv1.CronJob {
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
									Env:   schedule.Env,
								},
							},
						},
					},
				},
			},
		},
	}
}
