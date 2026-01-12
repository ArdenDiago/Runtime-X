package worker

import (
	"fmt"
	"os/exec"
	"time"
)

func RunJob(job *Job) {
	job.Status = StatusRunning
	job.StartedAt = time.Now()

	fmt.Println("Job: ", job.ID, " Status: ", job.Status)

	cmd := exec.Command("sh", "-c", job.Command)
	err := cmd.Run()

	job.EndedAt = time.Now()

	if err != nil {
		job.Status = StatusFailed
		job.Error = err.Error()
		fmt.Println("Job: ", job.ID, " Status: ", job.Status)
		return
	}

	job.Status = StatusCompleted
	fmt.Println("Job: ", job.ID, " Status: ", job.Status)

}
