package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	worker "runtimex/internal/core"
	"runtimex/internal/queue"

	"github.com/google/uuid"
)

type TaskHandler struct {
	queue *queue.JobQueue
}

func NewTaskHandler(q *queue.JobQueue) *TaskHandler {
	return &TaskHandler{
		queue: q,
	}
}

func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Command string `json:"command"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Command == "" {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	job := &worker.Job{
		ID:        uuid.New().String(),
		Command:   req.Command,
		Status:    worker.StatusQueued,
		CreatedAt: time.Now(),
	}

	if err := h.queue.Enqueue(job); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	// Intentionally simple for now
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]string{
		"listing not implemented yet",
	})
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
