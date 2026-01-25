package worker

import (
	"os/exec"
	"time"

	"runtimex/internal/core"
	"runtimex/internal/logging"
)

func RunJob(job *core.Job) {
	job.Status = core.StatusRunning
	job.StartedAt = time.Now()
	logging.LogJob(job)

	cmd := exec.Command("sh", "-c", job.Command)
	err := cmd.Run()

	job.EndedAt = time.Now()

	if err != nil {
		job.Status = core.StatusFailed
		job.Error = err.Error()
		logging.LogJob(job)
		return
	}

	job.Status = core.StatusCompleted
	logging.LogJob(job)
}
