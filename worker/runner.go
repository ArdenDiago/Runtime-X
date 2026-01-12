package worker

import (
	"os/exec"
	"time"
)

func RunJob(job *Job) {
	job.Status = StatusRunning
	job.StartedAt = time.Now()

	cmd := exec.Command("sh", "-c", job.Command)
	err := cmd.Run()

	job.EndedAt = time.Now()

	if err != nil {
		job.Status = StatusFailed
		job.Error = err.Error()
		return
	}

	job.Status = StatusCompleted
}
