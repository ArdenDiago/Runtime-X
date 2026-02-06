package main

import (
	"log"
	"net/http"
	"time"

	dockerRoutes "runtimex/cmd/api/docker"
	"runtimex/internal/queue"
	"runtimex/internal/worker"
)

func main() {
	// Initialize Docker job queue and worker pool
	dockerQueue := queue.NewDockerJobQueue(10)
	workerPool := worker.NewWorkerPool(3)

	// Start Docker job scheduler with 10 minute timeout per job
	worker.StartDockerScheduler(dockerQueue, workerPool, 10*time.Minute)
	log.Println("Docker job scheduler started")

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Welcome to our home page"))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Docker routes with queue integration
	mux.Handle("/docker/", dockerRoutes.RouterWithQueue(dockerQueue))

	log.Println("API started on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

