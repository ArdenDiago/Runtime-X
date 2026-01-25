package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"runtimex/api-service/internal/models"
	"runtimex/api-service/internal/scheduler"
	"runtimex/worker"

	"github.com/google/uuid"
)

type TaskHandler struct {
	Scheduler *scheduler.Scheduler
	Queue     *worker.JobQueue
}

func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Command string `json:"command"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Command == "" {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	task := models.Task{
		ID:        uuid.New().String(),
		Command:   req.Command,
		Status:    models.TaskPending,
		CreatedAt: time.Now(),
	}

	h.Scheduler.AddTask(task)

	// Enqueue job for immediate execution
	job := &worker.Job{
		ID:        task.ID,
		Command:   task.Command,
		Status:    worker.StatusQueued,
		CreatedAt: task.CreatedAt,
	}
	if err := h.Queue.Enqueue(job); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.Scheduler.ListTasks())
}

func (h *TaskHandler) ExecuteTask(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/execute")
	id := strings.TrimPrefix(path, "/tasks/")

	task, exists := h.Scheduler.GetTask(id)
	if !exists {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	task.Status = models.TaskRunning
	h.Scheduler.UpdateTask(task)

	job := &worker.Job{
		ID:        task.ID,
		Command:   task.Command,
		Status:    worker.StatusQueued,
		CreatedAt: task.CreatedAt,
	}

	if err := h.Queue.Enqueue(job); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("Task execution started"))
}
