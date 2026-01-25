package queue

import (
	"sync"

	"runtimex/internal/core"
)

type JobQueue struct {
	jobs     []*core.Job
	capacity int
	mu       sync.Mutex
	cond     *sync.Cond
}

func NewJobQueue(capacity int) *JobQueue {
	q := &JobQueue{
		jobs:     make([]*core.Job, 0),
		capacity: capacity,
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *JobQueue) Enqueue(job *core.Job) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.jobs) >= q.capacity {
		return core.ErrQueueFull
	}

	q.jobs = append(q.jobs, job)
	q.cond.Signal()
	return nil
}

func (q *JobQueue) Dequeue() *core.Job {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.jobs) == 0 {
		q.cond.Wait()
	}

	job := q.jobs[0]
	q.jobs = q.jobs[1:]
	return job
}
