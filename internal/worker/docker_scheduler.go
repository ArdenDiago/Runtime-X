package worker

import (
	"time"

	"runtimex/internal/queue"
)

// StartDockerScheduler starts the Docker job scheduler
// It continuously processes jobs from the queue using the worker pool
func StartDockerScheduler(q *queue.DockerJobQueue, pool *WorkerPool, timeout time.Duration) {
	runner := NewDockerRunner(timeout)

	go func() {
		for {
			config := q.Dequeue()

			pool.Acquire()
			go func() {
				defer pool.Release()
				runner.RunDockerJob(config)
			}()
		}
	}()
}
