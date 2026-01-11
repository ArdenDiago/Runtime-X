package main

import "time"

type JobStatus string

const (
	JobRunning   JobStatus = "running"
	JobCrashed   JobStatus = "crashed"
	JobCompleted JobStatus = "completed"
)

type Job struct {
	ID        string
	Command   string
	Args      []string
	Status    JobStatus
	StartedAt time.Time
	EndedAt   time.Time
	ExitCode  int
}
