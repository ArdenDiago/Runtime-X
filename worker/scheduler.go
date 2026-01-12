package worker

func StartScheduler(queue *JobQueue, pool *WorkerPool) {
	go func() {
		for {
			job := queue.Dequeue()

			pool.Acquire()

			go func(j *Job) {
				defer pool.Release()
				RunJob(j)
			}(job)
		}
	}()
}
