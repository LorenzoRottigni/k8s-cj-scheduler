package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SchedulerSpec defines the desired state of Scheduler
type SchedulerSpec struct {
	// Schedules is the list of scheduled jobs to create
	Schedules []Schedule `json:"schedules,omitempty"`
}

// Schedule defines a single cron job specification
type Schedule struct {
	// Name is a unique name for the schedule (used to identify the cronjob)
	Name string `json:"name"`

	// Image is the container image to run in the cronjob
	Image string `json:"image"`

	// CronExpression is the cron expression string that defines when to run the job
	CronExpression string `json:"cronExpression"`

	// Params is the array of command line arguments to pass to the container image
	Params []string `json:"params,omitempty"`

	// Env is a list of environment variables to set in the container
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`
}

// SchedulerStatus defines the observed state of Scheduler
type SchedulerStatus struct {
	// LastScheduleTime tracks the last time a job was successfully created for any schedule.
	// +optional
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`

	// Active holds the names of the currently running Jobs created by this Scheduler.
	// +optional
	Active []corev1.ObjectReference `json:"active,omitempty"`

	// Conditions store the status of the Scheduler in a Kubernetes friendly way.
	// This follows the standard Kubernetes API conventions.
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the most recent generation observed for this Scheduler.
	// It corresponds to the Scheduler's generation, which is updated on mutation
	// by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Scheduler is the Schema for the schedulers API
type Scheduler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SchedulerSpec   `json:"spec,omitempty"`
	Status SchedulerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SchedulerList contains a list of Scheduler
type SchedulerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Scheduler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Scheduler{}, &SchedulerList{})
}
