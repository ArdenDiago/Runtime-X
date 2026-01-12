# File Structure

## `main.go`
**Purpose**: Rutime bootstraper

**What it is responsible for:**
* Starting the worker process
* Initializing queue, scheduler, and worker pool
* Wiring components together
* Handling startup & graceful shutdown

**What it must NOT do:**
* Execute jobs
* Contain scheduling logic
* Know job execution details

Think of this as the orchestrator, not a decision-maker.

## `job.go`

**Purpose:** Job definition & lifecycle

**What it is responsible for:**
* Defining what a “job” is
* Job metadata (ID, type, command)
* Job states (queued, running, completed, failed)
* Timestamps & status transitions

**What it must NOT do:**
* Decide when jobs run
* Execute jobs
* Manage workers

This is your contract for work.

## `queue.go` (to be added / conceptual)

**Purpose:** Job ordering & buffering
* What it is responsible for:
* Holding pending jobs
* Enqueue / dequeue operations
* Blocking or waiting when empty

**What it must NOT do:**
* Know worker count
* Execute jobs
* Decide priorities (yet)

Queue = waiting room, nothing else.

## `scheduler.go` (to be added / conceptual)

**Purpose:** Decision engine

**What it is responsible for:**
* Selecting the next job from the queue
* Checking worker availability
* Enforcing concurrency limits
* Transitioning job state to RUNNING

**What it must NOT do:**
* Execute jobs
* Manage logs
* Know execution internals

Scheduler = brain, not hands.

## `pool.go` (to be added / conceptual)

**Purpose:** Concurrency controller

**What it is responsible for:**
* Managing worker slots
* Tracking busy vs idle workers
* Enforcing max parallel jobs
* Backpressure handling

**What it must NOT do:**
* Pick jobs
* Execute jobs
* Modify job data

Pool = traffic control.


## `runner.go`

**Purpose:** Single-job execution

**What it is responsible for:**
* Running exactly ONE job
* Executing command/script
* Capturing stdout/stderr
* Returning exit status

**What it must NOT do:**
* Decide which job runs
* Interact with queue
* Know about concurrency

Runner = hands, not brain.


## `logger.go`

**Purpose:** Observability

**What it is responsible for:**
* Structured logging
* Job-scoped logs
* Error reporting

**What it must NOT do:**
* Store job state
* Control execution flow

Logger = eyes, not control.

* 
