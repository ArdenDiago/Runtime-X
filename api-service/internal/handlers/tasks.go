package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"runtimex/api-service/internal/models"
	"runtimex/api-service/internal/scheduler"

	"github.com/google/uuid"
)

type TaskHandler struct {
	Scheduler *scheduler.Scheduler
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.Scheduler.ListTasks())
}
