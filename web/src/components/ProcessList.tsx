import { useState } from 'react'
import type { ProcessJSON, ProcessState } from '../api/types'
import { startProcess, stopProcess, deleteProcess, listProcesses } from '../api/client'
import { StatusBadge } from './StatusBadge'
import { usePolling } from '../hooks/usePolling'
import { ProcessForm } from './ProcessForm'

interface ProcessListProps {
  onSelect?: (name: string) => void
  selectedProcess?: string | null
}

function isStartable(state: ProcessState | undefined): boolean {
  return state === 'idle' || state === 'stopped' || state === 'failed'
}

function isStoppable(state: ProcessState | undefined): boolean {
  return state === 'running' || state === 'starting' || state === 'restarting'
}

export function ProcessList({ onSelect, selectedProcess }: ProcessListProps) {
  const [processes, setProcesses] = useState<ProcessJSON[]>([])
  const [error, setError] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [editTarget, setEditTarget] = useState<ProcessJSON | null>(null)

  // Fetch and refresh process list; called immediately and every 2 seconds.
  async function fetchProcesses() {
    try {
      const procs = await listProcesses()
      setProcesses(procs)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load processes')
    }
  }

  usePolling(fetchProcesses, 2000)

  async function handleStart(name: string, e: React.MouseEvent) {
    e.stopPropagation()
    setActionError(null)
    try {
      await startProcess(name)
      await fetchProcesses()
    } catch (err) {
      setActionError(`Start failed: ${err instanceof Error ? err.message : String(err)}`)
    }
  }

  async function handleStop(name: string, e: React.MouseEvent) {
    e.stopPropagation()
    setActionError(null)
    try {
      await stopProcess(name)
      await fetchProcesses()
    } catch (err) {
      setActionError(`Stop failed: ${err instanceof Error ? err.message : String(err)}`)
    }
  }

  async function handleDelete(name: string, e: React.MouseEvent) {
    e.stopPropagation()
    const confirmed = window.confirm(
      `Delete process "${name}"? This cannot be undone.`
    )
    if (!confirmed) return

    setActionError(null)
    try {
      await deleteProcess(name)
      if (selectedProcess === name && onSelect) {
        onSelect('')
      }
      await fetchProcesses()
    } catch (err) {
      setActionError(`Delete failed: ${err instanceof Error ? err.message : String(err)}`)
    }
  }

  if (editTarget) {
    return (
      <ProcessForm
        existing={editTarget}
        onSuccess={() => {
          setEditTarget(null)
          fetchProcesses()
        }}
        onCancel={() => setEditTarget(null)}
      />
    )
  }

  if (error) {
    return (
      <div style={{ padding: '1rem' }}>
        <p style={{ color: '#f44336' }}>Error: {error}</p>
        <button onClick={fetchProcesses}>Retry</button>
      </div>
    )
  }

  return (
    <div style={{ padding: '0.5rem 0' }}>
      {actionError && (
        <p style={{ color: '#f44336', marginBottom: '0.5rem' }}>{actionError}</p>
      )}
      {processes.length === 0 ? (
        <p style={{ color: '#666', fontStyle: 'italic' }}>
          No processes registered. Create one below.
        </p>
      ) : (
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.9em' }}>
          <thead>
            <tr style={{ borderBottom: '2px solid #ddd', textAlign: 'left' }}>
              <th style={{ padding: '6px 8px' }}>Name</th>
              <th style={{ padding: '6px 8px' }}>State</th>
              <th style={{ padding: '6px 8px' }}>Command</th>
              <th style={{ padding: '6px 8px' }}>Restarts</th>
              <th style={{ padding: '6px 8px' }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {processes.map(proc => {
              const state = (proc.state ?? 'idle') as ProcessState
              const isSelected = proc.name === selectedProcess
              return (
                <tr
                  key={proc.name}
                  style={{
                    borderBottom: '1px solid #eee',
                    backgroundColor: isSelected ? '#e8f5e9' : 'transparent',
                    cursor: 'pointer',
                  }}
                  onClick={() => onSelect?.(proc.name)}
                >
                  <td style={{ padding: '6px 8px', fontWeight: 600 }}>{proc.name}</td>
                  <td style={{ padding: '6px 8px' }}>
                    <StatusBadge state={state} />
                  </td>
                  <td style={{ padding: '6px 8px', fontFamily: 'monospace', fontSize: '0.85em' }}>
                    {proc.command}
                  </td>
                  <td style={{ padding: '6px 8px' }}>{proc.restart_count ?? 0}</td>
                  <td
                    style={{ padding: '6px 8px', display: 'flex', gap: '4px' }}
                    onClick={e => e.stopPropagation()}
                  >
                    {isStartable(state) && (
                      <button
                        onClick={e => handleStart(proc.name, e)}
                        style={{ backgroundColor: '#4caf50', color: '#fff', border: 'none' }}
                        title="Start"
                      >
                        Start
                      </button>
                    )}
                    {isStoppable(state) && (
                      <button
                        onClick={e => handleStop(proc.name, e)}
                        style={{ backgroundColor: '#ff9800', color: '#fff', border: 'none' }}
                        title="Stop"
                      >
                        Stop
                      </button>
                    )}
                    {(state === 'idle' || state === 'stopped' || state === 'failed') && (
                      <button
                        onClick={e => { e.stopPropagation(); setEditTarget(proc) }}
                        style={{ backgroundColor: '#607d8b', color: '#fff', border: 'none' }}
                        title="Edit"
                      >
                        Edit
                      </button>
                    )}
                    <button
                      onClick={e => handleDelete(proc.name, e)}
                      style={{ backgroundColor: '#f44336', color: '#fff', border: 'none' }}
                      title="Delete"
                    >
                      Delete
                    </button>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      )}
    </div>
  )
}
