import type { ProcessState } from '../api/types'

interface StatusBadgeProps {
  state: ProcessState
}

const STATE_STYLES: Record<ProcessState, React.CSSProperties> = {
  idle: { backgroundColor: '#9e9e9e', color: '#fff' },
  starting: { backgroundColor: '#2196f3', color: '#fff' },
  running: { backgroundColor: '#4caf50', color: '#fff' },
  stopping: { backgroundColor: '#ff9800', color: '#fff' },
  stopped: { backgroundColor: '#607d8b', color: '#fff' },
  failed: { backgroundColor: '#f44336', color: '#fff' },
  restarting: { backgroundColor: '#9c27b0', color: '#fff' },
}

const badgeStyle: React.CSSProperties = {
  display: 'inline-block',
  padding: '2px 8px',
  borderRadius: '12px',
  fontSize: '0.78em',
  fontWeight: 600,
  letterSpacing: '0.02em',
  textTransform: 'uppercase',
}

export function StatusBadge({ state }: StatusBadgeProps) {
  return (
    <span style={{ ...badgeStyle, ...(STATE_STYLES[state] ?? STATE_STYLES.idle) }}>
      {state}
    </span>
  )
}
