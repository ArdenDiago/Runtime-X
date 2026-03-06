import { useEffect, useRef, useState, useCallback } from 'react'
import type { LogEntry } from '../api/types'
import { getLogs } from '../api/client'
import { usePolling } from '../hooks/usePolling'

interface LogViewerProps {
  processName: string
}

const containerStyle: React.CSSProperties = {
  border: '1px solid #ddd',
  borderRadius: '4px',
  padding: '0.5rem',
  marginTop: '0.5rem',
}

const outputStyle: React.CSSProperties = {
  fontFamily: 'monospace',
  fontSize: '0.82em',
  maxHeight: '300px',
  overflowY: 'auto',
  backgroundColor: '#1e1e1e',
  color: '#d4d4d4',
  padding: '0.5rem',
  borderRadius: '2px',
}

export function LogViewer({ processName }: LogViewerProps) {
  const [entries, setEntries] = useState<LogEntry[]>([])
  const [error, setError] = useState<string | null>(null)
  const bottomRef = useRef<HTMLDivElement>(null)

  const fetchLogs = useCallback(async () => {
    try {
      const resp = await getLogs(processName)
      setEntries(resp.entries ?? [])
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load logs')
    }
  }, [processName])

  // Poll every 2 seconds for live log output.
  usePolling(fetchLogs, 2000)

  // Scroll to bottom whenever new entries arrive.
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [entries])

  return (
    <div style={containerStyle}>
      <h4 style={{ margin: '0 0 0.5rem 0' }}>Logs: {processName}</h4>
      {error && (
        <p style={{ color: '#f44336', margin: '0 0 0.5rem 0', fontSize: '0.85em' }}>
          {error}
        </p>
      )}
      <div style={outputStyle}>
        {entries.length === 0 && !error && (
          <span style={{ color: '#666', fontStyle: 'italic' }}>No log output yet.</span>
        )}
        {entries.map((entry, i) => (
          <div
            key={i}
            style={{ color: entry.stream === 'stderr' ? '#f48771' : '#d4d4d4' }}
          >
            <span style={{ color: '#608b4e', marginRight: '0.5rem' }}>
              {new Date(entry.timestamp).toLocaleTimeString()}
            </span>
            <span style={{ color: entry.stream === 'stderr' ? '#f48771' : '#9cdcfe', marginRight: '0.5rem' }}>
              [{entry.stream}]
            </span>
            {entry.text}
          </div>
        ))}
        <div ref={bottomRef} />
      </div>
    </div>
  )
}
