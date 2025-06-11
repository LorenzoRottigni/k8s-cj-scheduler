// api/v1/scheduler_types.go
package v1

import (
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
}

// SchedulerStatus defines the observed state of Scheduler
type SchedulerStatus struct {
	// You can add status fields here if needed
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
