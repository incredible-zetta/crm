package domain

import "time"

// TaskKind represents the kind of a scheduled task.
type TaskKind string

const (
	// TaskEmail represents a single email task.
	TaskEmail TaskKind = "email"
	// TaskCampaign represents a campaign delivery task.
	TaskCampaign TaskKind = "campaign"
	// TaskEmailAudit represents a batch email verification task.
	TaskEmailAudit TaskKind = "email_audit"
)

// TaskKinds contains all valid task kinds.
var TaskKinds = []TaskKind{TaskEmail, TaskCampaign, TaskEmailAudit}

// Valid checks if the task kind is valid.
func (k TaskKind) Valid() bool {
	for _, kind := range TaskKinds {
		if k == kind {
			return true
		}
	}
	return false
}

// TaskStatus represents the execution status of a scheduled task.
type TaskStatus string

const (
	// TaskPending indicates the task is waiting to run.
	TaskPending TaskStatus = "pending"
	// TaskRunning indicates the task is currently running.
	TaskRunning TaskStatus = "running"
	// TaskDone indicates the task has completed successfully.
	TaskDone TaskStatus = "done"
	// TaskFailed indicates the task failed to run successfully.
	TaskFailed TaskStatus = "failed"
	// TaskCancelled indicates the task has been cancelled.
	TaskCancelled TaskStatus = "cancelled"
)

// TaskStatuses contains all valid task statuses.
var TaskStatuses = []TaskStatus{TaskPending, TaskRunning, TaskDone, TaskFailed, TaskCancelled}

// Valid checks if the task status is valid.
func (s TaskStatus) Valid() bool {
	for _, status := range TaskStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// ScheduledTask represents an asynchronous background job.
type ScheduledTask struct {
	ID        int64
	TenantID  string
	Kind      TaskKind
	Payload   map[string]any
	RunAt     time.Time
	Status    TaskStatus
	Attempts  int
	LastError string
	CreatedAt time.Time
}
