package scheduler

import (
	"runtimex/api-service/internal/models"
	"sync"
)

type Scheduler struct {
	mu    sync.Mutex
	tasks map[string]models.Task
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		tasks: make(map[string]models.Task),
	}
}

func (s *Scheduler) AddTask(task models.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[task.ID] = task
}

func (s *Scheduler) ListTasks() []models.Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]models.Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		result = append(result, task)
	}
	return result
}
