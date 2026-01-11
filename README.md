# Runtime X

> A backend-first runtime system for scheduling, executing, and tracking jobs with reliability and clear service boundaries.

Runtime X is a collaborative Go-based project focused on system design, service orchestration, and execution reliability.  
It is intentionally designed as a backend-first system, where the UI acts only as a control and observability layer.

---

## 🧠 Project Philosophy

Runtime X follows a simple but strict philosophy:

Build the system first.  
Observe it second.  
Beautify it last.

- The backend is the source of truth
- Services are independent and explicit
- The UI contains no business logic
- Every job can be managed via APIs alone

If the UI is removed and the system still works via `curl`, the design is correct.

---

## 🎯 What Problem Does Runtime X Solve?

Runtime X provides a structured way to:
- Accept jobs
- Schedule execution
- Execute tasks
- Track execution state
- Retry failed jobs

The focus is on reliability, clarity, and extensibility — not UI complexity.

---

## 🧩 Core Concepts

### Job Lifecycle

Each job moves through clearly defined states:

PENDING → RUNNING → COMPLETED  
PENDING → RUNNING → FAILED → RETRYING → RUNNING

- All state transitions are explicit
- Failures are detected and recorded
- Retries follow defined policies

---

## 🏗️ Architecture Overview

Runtime X is built using independent services:

| Service   | Responsibility |
|----------|----------------|
| Scheduler | Accepts jobs, tracks state, decides retries |
| Executor  | Executes jobs and reports results |
| Gateway   | Exposes APIs and routes requests |
| UI        | Displays state and triggers actions |

Services communicate via HTTP APIs with clear contracts.

---

## ⚙️ Services Description

### Scheduler Service
- Accepts job submissions
- Maintains job state
- Manages retry logic
- Acts as the system’s control plane

### Executor Service
- Executes assigned jobs
- Reports success or failure
- Handles execution errors
- Contains no scheduling logic

### Gateway Service
- Entry point for UI and external clients
- Routes requests to internal services
- Aggregates responses when needed

### UI (Minimal)
- Job creation form
- Job status table
- Job detail view
- System health indicators

The UI is intentionally minimal and optional.

---

## 🧪 Failure Handling & Reliability

In Runtime X (v1):
- A failure means execution returned an error or exited unexpectedly
- Failed jobs are tracked explicitly
- Retry policies define:
  - Maximum retries
  - Retry conditions
  - Final failure state

Runtime X does not act as a full OS or container supervisor in v1.  
Those capabilities are intentionally deferred.

---

## 🛠️ Tech Stack

- Language: Go
- Communication: HTTP (REST)
- Containerization: Docker
- Local orchestration: Docker Compose
- CI: GitHub Actions
- UI: Minimal React-based interface (optional)

---

## 📁 Repository Structure

