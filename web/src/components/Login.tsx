import { useState } from 'react'
import { login } from '../api/client'

export function Login({ onLogin }: { onLogin: () => void }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await login({ username, password })
      onLogin()
    } catch (err: any) {
      setError(err.message || 'Login failed')
    }
  }

  return (
    <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh', backgroundColor: '#f0f2f5' }}>
      <div style={{ padding: '2.5rem', backgroundColor: 'white', borderRadius: '8px', boxShadow: '0 4px 6px rgba(0,0,0,0.1)', width: '100%', maxWidth: '400px' }}>
        <h2 style={{ textAlign: 'center', margin: '0 0 1.5rem', color: '#1a237e', fontSize: '1.8rem' }}>Runtime-X</h2>
        <p style={{ textAlign: 'center', color: '#555', marginBottom: '2rem' }}>Sign in to manage processes</p>
        
        {error && (
          <div style={{ color: '#d32f2f', backgroundColor: '#ffebee', padding: '0.75rem', borderRadius: '4px', marginBottom: '1.5rem', textAlign: 'center', border: '1px solid #ef9a9a' }}>
            {error}
          </div>
        )}
        
        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
          <div>
            <label style={{ display: 'block', marginBottom: '0.5rem', color: '#333', fontWeight: 500 }}>Username</label>
            <input 
              type="text" 
              value={username} 
              onChange={e => setUsername(e.target.value)} 
              style={{ width: '100%', padding: '0.75rem', border: '1px solid #ccc', borderRadius: '4px', boxSizing: 'border-box', fontSize: '1rem' }} 
              required 
            />
          </div>
          <div>
            <label style={{ display: 'block', marginBottom: '0.5rem', color: '#333', fontWeight: 500 }}>Password</label>
            <input 
              type="password" 
              value={password} 
              onChange={e => setPassword(e.target.value)} 
              style={{ width: '100%', padding: '0.75rem', border: '1px solid #ccc', borderRadius: '4px', boxSizing: 'border-box', fontSize: '1rem' }} 
              required 
            />
          </div>
          <button 
            type="submit" 
            style={{ padding: '0.85rem', backgroundColor: '#2196f3', color: 'white', border: 'none', borderRadius: '4px', fontSize: '1rem', fontWeight: 'bold', cursor: 'pointer', marginTop: '0.5rem', transition: 'background-color 0.2s' }}
          >
            Login
          </button>
        </form>
      </div>
    </div>
  )
}
