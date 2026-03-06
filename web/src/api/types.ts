// Types matching Go structs from internal/scheduler and internal/api.
// Field names use snake_case to match the JSON serialization in handlers.go.

export type ProcessState =
  | 'idle'
  | 'starting'
  | 'running'
  | 'stopping'
  | 'stopped'
  | 'failed'
  | 'restarting'

export interface RestartPolicyJSON {
  mode: 'never' | 'on-failure' | 'always'
  max_retries: number
  delay_secs: number
  max_delay_secs: number
  backoff_factor: number
}

// ProcessJSON matches the processJSON struct in handlers.go.
// Used for both requests (POST/PUT) and responses (GET).
export interface ProcessJSON {
  name: string
  command: string
  args?: string[]
  env?: string[]
  work_dir?: string
  restart_policy: RestartPolicyJSON
  depends_on?: string[]
  log_buffer_size?: number
  stop_timeout_secs?: number

  // Runtime fields -- present in GET responses only.
  state?: ProcessState
  restart_count?: number
}

// API envelope -- all responses are wrapped in { data: T } or { error: string }
export interface APIEnvelope<T> {
  data?: T
  error?: string
}

// LogEntry matches logEntryJSON in handlers.go.
export interface LogEntry {
  timestamp: string
  stream: 'stdout' | 'stderr'
  text: string
}

// LogsEnvelope matches logsEnvelope in handlers.go.
export interface LogsEnvelope {
  name: string
  entries: LogEntry[]
}
