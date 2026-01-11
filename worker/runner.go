package main

import (
	"log"
	"os/exec"
	"time"
)

func RunJob(job *Job) {
	log.Printf("Starting job %s\n", job.ID)

	job.Status = JobRunning
	job.StartedAt = time.Now()

	cmd := exec.Command(job.Command, job.Args...)

	err := cmd.Run()

	job.EndedAt = time.Now()

	if err != nil {
		job.Status = JobCrashed

		if exitErr, ok := err.(*exec.ExitError); ok {
			job.ExitCode = exitErr.ExitCode()
		}

		log.Printf("Job %s crashed with exit code %d\n", job.ID, job.ExitCode)
		return
	}

	job.Status = JobCompleted
	job.ExitCode = 0
	log.Printf("Job %s completed successfully\n", job.ID)
}
