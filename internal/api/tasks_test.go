package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"runtimex/api-service/internal/models"
	"runtimex/api-service/internal/scheduler"
	"runtimex/worker"
)

func TestCreateTask(t *testing.T) {
	// Setup
	sched := scheduler.NewScheduler()
	queue := worker.NewJobQueue(5)

	handler := &TaskHandler{
		Scheduler: sched,
		Queue:     queue,
	}

	// Request body
	body := []byte(`{"command":"echo test"}`)
	req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	// Response recorder
	rr := httptest.NewRecorder()

	// Call handler
	handler.CreateTask(rr, req)

	// Assertions
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var task models.Task
	if err := json.NewDecoder(rr.Body).Decode(&task); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if task.Command != "echo test" {
		t.Fatalf("expected command 'echo test', got '%s'", task.Command)
	}

	if task.Status != models.TaskPending {
		t.Fatalf("expected status PENDING, got %s", task.Status)
	}
}

func TestListTasks(t *testing.T) {
	sched := scheduler.NewScheduler()
	queue := worker.NewJobQueue(5)

	handler := &TaskHandler{
		Scheduler: sched,
		Queue:     queue,
	}

	// Pre-add a task
	task := models.Task{
		ID:      "1",
		Command: "echo hello",
		Status:  models.TaskPending,
	}
	sched.AddTask(task)

	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	rr := httptest.NewRecorder()

	handler.ListTasks(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var tasks []models.Task
	if err := json.NewDecoder(rr.Body).Decode(&tasks); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
}

func TestExecuteTask(t *testing.T) {
	sched := scheduler.NewScheduler()
	queue := worker.NewJobQueue(5)

	handler := &TaskHandler{
		Scheduler: sched,
		Queue:     queue,
	}

	// Add task
	task := models.Task{
		ID:      "exec-1",
		Command: "echo test",
		Status:  models.TaskPending,
	}
	sched.AddTask(task)

	req := httptest.NewRequest(
		http.MethodPost,
		"/tasks/exec-1/execute",
		nil,
	)
	rr := httptest.NewRecorder()

	handler.ExecuteTask(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", rr.Code)
	}

	updatedTask, _ := sched.GetTask("exec-1")
	if updatedTask.Status != models.TaskRunning {
		t.Fatalf("expected status RUNNING, got %s", updatedTask.Status)
	}
}
