package main

import (
	"log"

	"runtimex/internal/queue"
	"runtimex/internal/worker"
)

func main() {
	log.Println("Worker started")

	q := queue.NewJobQueue(10)
	pool := worker.NewWorkerPool(3)

	worker.StartScheduler(q, pool)

	select {}
}
