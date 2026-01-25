package logging

import (
	"log"

	"runtimex/internal/core"
)

func LogJob(job *core.Job) {
	log.Printf(
		"Job=%s Status=%s Error=%s",
		job.ID,
		job.Status,
		job.Error,
	)
}
