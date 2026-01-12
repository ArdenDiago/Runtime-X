package worker

import "errors"

var (
	ErrQueueFull = errors.New("job queue is full")
)
