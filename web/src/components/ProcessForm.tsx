import { useState } from 'react'
import type { ProcessJSON } from '../api/types'
import { createProcess, updateProcess } from '../api/client'

interface ProcessFormProps {
  /** If provided, editing an existing process definition (PUT). Otherwise, creating (POST). */
  existing?: ProcessJSON
  onSuccess: () => void
  onCancel: () => void
}

interface FormState {
  name: string
  command: string
  args: string
  workDir: string
  dependsOn: string
  restartMode: string
  maxRetries: string
  delaySecs: string
}

function processToForm(proc: ProcessJSON): FormState {
  return {
    name: proc.name,
    command: proc.command,
    args: (proc.args ?? []).join(' '),
    workDir: proc.work_dir ?? '',
    dependsOn: (proc.depends_on ?? []).join(', '),
    restartMode: proc.restart_policy?.mode ?? 'never',
    maxRetries: String(proc.restart_policy?.max_retries ?? 0),
    delaySecs: String(proc.restart_policy?.delay_secs ?? 1),
  }
}

const DEFAULT_FORM: FormState = {
  name: '',
  command: '',
  args: '',
  workDir: '',
  dependsOn: '',
  restartMode: 'never',
  maxRetries: '0',
  delaySecs: '1',
}

const labelStyle: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: '2px',
  fontSize: '0.9em',
  fontWeight: 500,
}

const inputStyle: React.CSSProperties = {
  marginTop: '2px',
  width: '100%',
}

export function ProcessForm({ existing, onSuccess, onCancel }: ProcessFormProps) {
  const [form, setForm] = useState<FormState>(
    existing ? processToForm(existing) : DEFAULT_FORM
  )
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const isEdit = Boolean(existing)

  function handleChange(e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) {
    setForm(prev => ({ ...prev, [e.target.name]: e.target.value }))
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setSubmitting(true)

    const def: Partial<ProcessJSON> = {
      name: form.name.trim(),
      command: form.command.trim(),
      args: form.args.trim() ? form.args.trim().split(/\s+/) : [],
      work_dir: form.workDir.trim() || undefined,
      depends_on: form.dependsOn
        .split(',')
        .map(s => s.trim())
        .filter(Boolean),
      restart_policy: {
        mode: form.restartMode as 'never' | 'on-failure' | 'always',
        max_retries: Number(form.maxRetries),
        delay_secs: Number(form.delaySecs),
        max_delay_secs: 60,
        backoff_factor: 2,
      },
    }

    try {
      if (isEdit && existing) {
        await updateProcess(existing.name, def)
      } else {
        await createProcess(def)
      }
      onSuccess()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form
      onSubmit={handleSubmit}
      style={{ display: 'flex', flexDirection: 'column', gap: '12px', maxWidth: '480px' }}
    >
      <h3 style={{ margin: 0 }}>{isEdit ? `Edit: ${existing!.name}` : 'New Process'}</h3>

      {error && (
        <p style={{ color: '#f44336', margin: 0, fontSize: '0.85em' }}>{error}</p>
      )}

      <label style={labelStyle}>
        Name
        <input
          name="name"
          value={form.name}
          onChange={handleChange}
          required
          disabled={isEdit}
          placeholder="my-process"
          style={inputStyle}
        />
      </label>

      <label style={labelStyle}>
        Command
        <input
          name="command"
          value={form.command}
          onChange={handleChange}
          required
          placeholder="/usr/bin/sleep"
          style={inputStyle}
        />
      </label>

      <label style={labelStyle}>
        Args (space-separated)
        <input
          name="args"
          value={form.args}
          onChange={handleChange}
          placeholder="30"
          style={inputStyle}
        />
      </label>

      <label style={labelStyle}>
        Working Directory
        <input
          name="workDir"
          value={form.workDir}
          onChange={handleChange}
          placeholder="/home/user/app"
          style={inputStyle}
        />
      </label>

      <label style={labelStyle}>
        Dependencies (comma-separated names)
        <input
          name="dependsOn"
          value={form.dependsOn}
          onChange={handleChange}
          placeholder="db, cache"
          style={inputStyle}
        />
      </label>

      <label style={labelStyle}>
        Restart Mode
        <select name="restartMode" value={form.restartMode} onChange={handleChange} style={inputStyle}>
          <option value="never">never</option>
          <option value="on-failure">on-failure</option>
          <option value="always">always</option>
        </select>
      </label>

      {form.restartMode !== 'never' && (
        <>
          <label style={labelStyle}>
            Max Retries (0 = unlimited)
            <input
              name="maxRetries"
              type="number"
              min="0"
              value={form.maxRetries}
              onChange={handleChange}
              style={inputStyle}
            />
          </label>
          <label style={labelStyle}>
            Initial Delay (seconds)
            <input
              name="delaySecs"
              type="number"
              min="0"
              step="0.1"
              value={form.delaySecs}
              onChange={handleChange}
              style={inputStyle}
            />
          </label>
        </>
      )}

      <div style={{ display: 'flex', gap: '8px', marginTop: '4px' }}>
        <button
          type="submit"
          disabled={submitting}
          style={{ backgroundColor: '#2196f3', color: '#fff', border: 'none' }}
        >
          {submitting ? 'Saving...' : isEdit ? 'Update' : 'Create'}
        </button>
        <button type="button" onClick={onCancel} style={{ border: '1px solid #ccc' }}>
          Cancel
        </button>
      </div>
    </form>
  )
}
