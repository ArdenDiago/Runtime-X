package queue

import (
	"sync"

	"runtimex/internal/core"
)

// DockerJobQueue manages a queue of Docker jobs
type DockerJobQueue struct {
	jobs     []*core.DockerfileConfig
	capacity int
	mu       sync.Mutex
	cond     *sync.Cond
}

// NewDockerJobQueue creates a new DockerJobQueue with specified capacity
func NewDockerJobQueue(capacity int) *DockerJobQueue {
	q := &DockerJobQueue{
		jobs:     make([]*core.DockerfileConfig, 0),
		capacity: capacity,
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// Enqueue adds a Docker job to the queue
func (q *DockerJobQueue) Enqueue(config *core.DockerfileConfig) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.jobs) >= q.capacity {
		return core.ErrQueueFull
	}

	q.jobs = append(q.jobs, config)
	q.cond.Signal()
	return nil
}

// Dequeue removes and returns the next Docker job from the queue
// Blocks if queue is empty
func (q *DockerJobQueue) Dequeue() *core.DockerfileConfig {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.jobs) == 0 {
		q.cond.Wait()
	}

	job := q.jobs[0]
	q.jobs = q.jobs[1:]
	return job
}

// Size returns the current number of jobs in the queue
func (q *DockerJobQueue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.jobs)
}
