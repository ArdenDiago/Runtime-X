import { useState } from 'react'
import './index.css'
import { ProcessList } from './components/ProcessList'
import { ProcessForm } from './components/ProcessForm'
import { LogViewer } from './components/LogViewer'

export default function App() {
  const [selectedProcess, setSelectedProcess] = useState<string | null>(null)
  const [showCreateForm, setShowCreateForm] = useState(false)
  // Key used to force ProcessList re-mount after create, triggering immediate poll.
  const [listKey, setListKey] = useState(0)

  function handleSelect(name: string) {
    setSelectedProcess(name || null)
  }

  function handleCreateSuccess() {
    setShowCreateForm(false)
    setListKey(k => k + 1)
  }

  return (
    <div style={{ maxWidth: '1100px', margin: '0 auto', padding: '1.5rem' }}>
      <header style={{ marginBottom: '1.5rem', borderBottom: '1px solid #ddd', paddingBottom: '0.75rem' }}>
        <h1 style={{ margin: 0, fontSize: '1.5rem' }}>Runtime-X Dashboard</h1>
        <p style={{ margin: '0.25rem 0 0', color: '#666', fontSize: '0.85em' }}>
          Process lifecycle manager
        </p>
      </header>

      <main>
        <section style={{ marginBottom: '2rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.75rem' }}>
            <h2 style={{ margin: 0, fontSize: '1.1rem' }}>Processes</h2>
            {!showCreateForm && (
              <button
                onClick={() => setShowCreateForm(true)}
                style={{ backgroundColor: '#2196f3', color: '#fff', border: 'none' }}
              >
                + New Process
              </button>
            )}
          </div>

          {showCreateForm && (
            <div style={{ marginBottom: '1.5rem', padding: '1rem', border: '1px solid #2196f3', borderRadius: '6px' }}>
              <ProcessForm
                onSuccess={handleCreateSuccess}
                onCancel={() => setShowCreateForm(false)}
              />
            </div>
          )}

          <ProcessList
            key={listKey}
            onSelect={handleSelect}
            selectedProcess={selectedProcess}
          />
        </section>

        {selectedProcess && (
          <section>
            <LogViewer processName={selectedProcess} />
          </section>
        )}
      </main>
    </div>
  )
}
