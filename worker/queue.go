package worker

import "sync"

type JobQueue struct {
	queue    []*Job
	capacity int
	mu       sync.Mutex
	cond     *sync.Cond
}

func NewJobQueue(capacity int) *JobQueue {
	q := &JobQueue{
		queue:    make([]*Job, 0),
		capacity: capacity,
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *JobQueue) Enqueue(job *Job) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.queue) >= q.capacity {
		return ErrQueueFull
	}

	q.queue = append(q.queue, job)
	q.cond.Signal()
	return nil
}

func (q *JobQueue) Dequeue() *Job {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.queue) == 0 {
		q.cond.Wait()
	}

	job := q.queue[0]
	q.queue = q.queue[1:]
	return job
}
