package main

import (
	"time"

	"runtimex/worker"
)

func main() {
	queue := worker.NewJobQueue(10)
	pool := worker.NewWorkerPool(3)

	worker.StartScheduler(queue, pool)

	queue.Enqueue(&worker.Job{
		ID:        "1",
		Command:   "echo Job 1",
		Status:    worker.StatusQueued,
		CreatedAt: time.Now(),
	})

	queue.Enqueue(&worker.Job{
		ID:        "2",
		Command:   "sleep 2 && echo Job 2",
		Status:    worker.StatusQueued,
		CreatedAt: time.Now(),
	})

	queue.Enqueue(&worker.Job{
		ID:        "3",
		Command:   "echo Job 3",
		Status:    worker.StatusQueued,
		CreatedAt: time.Now(),
	})

	time.Sleep(10 * time.Second)
}
