package main

import (
	"log"
	"net/http"
	"strings"

	"runtimex/api-service/internal/handlers"
	"runtimex/api-service/internal/scheduler"
	"runtimex/worker"
)

func main() {
	log.Println("API Service starting on :8080")

	// Initialize scheduler
	sched := scheduler.NewScheduler()

	// Initialize worker engine
	jobQueue := worker.NewJobQueue(10)
	workerPool := worker.NewWorkerPool(3)
	worker.StartScheduler(jobQueue, workerPool)

	// Initialize handlers
	taskHandler := &handlers.TaskHandler{
		Scheduler: sched,
		Queue:     jobQueue,
	}

	// Routes
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Welcome Page"))
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	http.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			taskHandler.CreateTask(w, r)
		case http.MethodGet:
			taskHandler.ListTasks(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/execute") {
			taskHandler.ExecuteTask(w, r)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})

	// Start server (LAST)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
