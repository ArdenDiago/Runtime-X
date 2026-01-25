package worker

import "runtimex/internal/queue"

func StartScheduler(q *queue.JobQueue, pool *WorkerPool) {
	go func() {
		for {
			job := q.Dequeue()

			pool.Acquire()
			go func() {
				defer pool.Release()
				RunJob(job)
			}()
		}
	}()
}
