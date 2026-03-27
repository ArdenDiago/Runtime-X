import { useState } from 'react'
import { ProcessList } from './ProcessList'
import { ProcessForm } from './ProcessForm'
import { LogViewer } from './LogViewer'
import { logout } from '../api/client'

export function Dashboard({ onLogout }: { onLogout: () => void }) {
  const [selectedProcess, setSelectedProcess] = useState<string | null>(null)
  const [showCreateForm, setShowCreateForm] = useState(false)
  const [refreshKey, setRefreshKey] = useState(0)

  function handleSelect(name: string) {
    setSelectedProcess(name || null)
    if (showCreateForm) setShowCreateForm(false)
  }

  function handleCreated() {
    setShowCreateForm(false)
    setRefreshKey(k => k + 1) // force ProcessList remount to refresh
  }

  function handleLogout() {
    logout()
      .then(onLogout)
      .catch(console.error)
  }

  return (
    <div style={{ maxWidth: '1100px', margin: '0 auto', padding: '1.5rem' }}>
      <header style={{ marginBottom: '1.5rem', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <h1 style={{ margin: 0, fontSize: '1.6rem', color: '#1a237e' }}>
            Runtime-X
          </h1>
          <p style={{ margin: '4px 0 0', color: '#555', fontSize: '0.9em' }}>
            Process Manager Dashboard
          </p>
        </div>
        <button 
          onClick={handleLogout} 
          style={{ padding: '8px 16px', backgroundColor: '#e53935', color: 'white', border: 'none', borderRadius: '4px', cursor: 'pointer', fontWeight: 'bold' }}
        >
          Logout
        </button>
      </header>

      <section>
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            marginBottom: '0.75rem',
          }}
        >
          <h2 style={{ margin: 0, fontSize: '1.1rem' }}>Processes</h2>
          <button
            onClick={() => setShowCreateForm(v => !v)}
            style={{
              backgroundColor: showCreateForm ? '#607d8b' : '#2196f3',
              color: '#fff',
              border: 'none',
              padding: '6px 14px',
              borderRadius: '4px',
              cursor: 'pointer',
            }}
          >
            {showCreateForm ? 'Cancel' : '+ New Process'}
          </button>
        </div>

        {showCreateForm && (
          <div
            style={{
              padding: '1rem',
              border: '1px solid #ddd',
              borderRadius: '4px',
              marginBottom: '1rem',
              backgroundColor: '#fafafa',
            }}
          >
            <ProcessForm
              onSuccess={handleCreated}
              onCancel={() => setShowCreateForm(false)}
            />
          </div>
        )}

        {/* key prop forces remount when refreshKey changes */}
        <ProcessList
          key={refreshKey}
          selectedProcess={selectedProcess}
          onSelect={handleSelect}
        />
      </section>

      {selectedProcess && (
        <section style={{ marginTop: '1.5rem' }}>
          <LogViewer processName={selectedProcess} />
        </section>
      )}
    </div>
  )
}
