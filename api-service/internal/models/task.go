package models

import "time"

type TaskStatus string

const (
	TaskPending   TaskStatus = "PENDING"
	TaskRunning   TaskStatus = "RUNNING"
	TaskCompleted TaskStatus = "COMPLETED"
	TaskFailed    TaskStatus = "FAILED"
	TaskRetrying  TaskStatus = "RETRYING"
)

type Task struct {
	ID        string     `json:"id"`
	Command   string     `json:"command"`
	Status    TaskStatus `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
}
