// API client for Runtime-X REST API.
// All requests go to /api/... — proxied to http://localhost:8080 in dev.
// Responses are wrapped in an APIEnvelope: { data?: T, error?: string }.

import type { ProcessJSON, LogsEnvelope, APIEnvelope } from './types'

const BASE = '/api'

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    ...options,
  })

  const envelope = await res.json() as APIEnvelope<T>

  if (!res.ok || envelope.error) {
    throw new Error(envelope.error ?? `HTTP ${res.status}`)
  }

  return envelope.data as T
}

// GET /api/processes -- list all registered processes
export function listProcesses(): Promise<ProcessJSON[]> {
  return request<ProcessJSON[]>('/processes')
}

// POST /api/processes -- register a new process
export function createProcess(def: Partial<ProcessJSON>): Promise<ProcessJSON> {
  return request<ProcessJSON>('/processes', {
    method: 'POST',
    body: JSON.stringify(def),
  })
}

// POST /api/processes with dry_run -- validate without persisting
export function dryRunProcess(def: Partial<ProcessJSON>): Promise<ProcessJSON> {
  return request<ProcessJSON>('/processes', {
    method: 'POST',
    body: JSON.stringify({ ...def, dry_run: true }),
  })
}

// GET /api/processes/{name} -- get a single process
export function getProcess(name: string): Promise<ProcessJSON> {
  return request<ProcessJSON>(`/processes/${encodeURIComponent(name)}`)
}

// PUT /api/processes/{name} -- update process definition (must be stopped)
export function updateProcess(name: string, def: Partial<ProcessJSON>): Promise<ProcessJSON> {
  return request<ProcessJSON>(`/processes/${encodeURIComponent(name)}`, {
    method: 'PUT',
    body: JSON.stringify(def),
  })
}

// DELETE /api/processes/{name} -- remove process (must be stopped)
export function deleteProcess(name: string): Promise<void> {
  return request<void>(`/processes/${encodeURIComponent(name)}`, { method: 'DELETE' })
}

// POST /api/processes/{name}/start -- start a process
export function startProcess(name: string): Promise<ProcessJSON> {
  return request<ProcessJSON>(`/processes/${encodeURIComponent(name)}/start`, { method: 'POST' })
}

// POST /api/processes/{name}/stop -- stop a running process
export function stopProcess(name: string): Promise<ProcessJSON> {
  return request<ProcessJSON>(`/processes/${encodeURIComponent(name)}/stop`, { method: 'POST' })
}

// GET /api/processes/{name}/logs -- get buffered log entries
export function getLogs(name: string): Promise<LogsEnvelope> {
  return request<LogsEnvelope>(`/processes/${encodeURIComponent(name)}/logs`)
}

// POST /api/login -- authenticate user
export function login(credentials: Record<string, string>): Promise<{message: string}> {
  return request<{message: string}>('/login', {
    method: 'POST',
    body: JSON.stringify(credentials)
  })
}

// POST /api/logout -- end session
export function logout(): Promise<{message: string}> {
  return request<{message: string}>('/logout', { method: 'POST' })
}

// GET /api/auth/check -- verify active session
export function checkAuth(): Promise<{authenticated: boolean}> {
  return request<{authenticated: boolean}>('/auth/check')
}
