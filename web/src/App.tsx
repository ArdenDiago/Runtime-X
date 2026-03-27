import { useState, useEffect } from 'react'
import './index.css'
import { Dashboard } from './components/Dashboard'
import { Login } from './components/Login'
import { checkAuth } from './api/client'

export default function App() {
  const [isAuthenticated, setIsAuthenticated] = useState<boolean | null>(null)

  useEffect(() => {
    checkAuth()
      .then(() => setIsAuthenticated(true))
      .catch(() => setIsAuthenticated(false))
  }, [])

  if (isAuthenticated === null) {
    return <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', fontFamily: 'sans-serif' }}>Loading...</div>
  }

  return isAuthenticated ? (
    <Dashboard onLogout={() => setIsAuthenticated(false)} />
  ) : (
    <Login onLogin={() => setIsAuthenticated(true)} />
  )
}
