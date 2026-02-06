package worker

type WorkerPool struct {
	sem chan struct{}
}

func NewWorkerPool(size int) *WorkerPool {
	return &WorkerPool{
		sem: make(chan struct{}, size),
	}
}

func (p *WorkerPool) Acquire() {
	p.sem <- struct{}{}
}

func (p *WorkerPool) Release() {
	<-p.sem
}
