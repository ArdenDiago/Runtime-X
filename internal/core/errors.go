package core

import "errors"

var ErrQueueFull = errors.New("job queue is full")
var ErrWorkerPoolExhausted = errors.New("no available workers in the pool")
var ErrJobNotFound = errors.New("job not found")
var ErrInvalidJobStatus = errors.New("invalid job status")
var ErrJobExecutionFailed = errors.New("job execution failed")
var ErrDatabaseConnection = errors.New("failed to connect to the database")
var ErrUnauthorizedAccess = errors.New("unauthorized access attempt")
var ErrInvalidJobData = errors.New("invalid job data provided")
var ErrTimeout = errors.New("operation timed out")
var ErrInternalServer = errors.New("internal server error")
var ErrDependencyFailure = errors.New("external dependency failure")
