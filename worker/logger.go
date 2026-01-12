package worker

import (
	"log"
)

func LogJob(job *Job) {
	log.Printf(
		"Job %s | Status=%s | Error=%s",
		job.ID,
		job.Status,
		job.Error,
	)
}
