package handlers

import (
	"encoding/json"
	"net/http"
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
