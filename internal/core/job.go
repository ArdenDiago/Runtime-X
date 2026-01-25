package core

import "time"

type JobStatus string

const (
	StatusQueued    JobStatus = "QUEUED"
	StatusRunning   JobStatus = "RUNNING"
	StatusCompleted JobStatus = "COMPLETED"
	StatusFailed    JobStatus = "FAILED"
)

type Job struct {
	ID        string
	Command   string
	Status    JobStatus
	CreatedAt time.Time
	StartedAt time.Time
	EndedAt   time.Time
	Error     string
}
