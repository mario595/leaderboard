import { ApiError, getJSON, type Me } from '../api/api'
import { useState, useEffect, type ReactNode } from 'react'
import { AuthContext, type AuthState } from './useAuth'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ status: 'loading' })
  useEffect(() => {
    let cancelled = false
    getJSON<Me>('/api/me')
      .then((user) => {
        if (!cancelled) setState({ status: 'user', user })
      })
      .catch((err: unknown) => {
        if (cancelled) return
        if (err instanceof ApiError && err.status === 401) {
          setState({ status: 'anonymous' })
        } else {
          setState({ status: 'error' })
        }
      })
    return () => {
      cancelled = true
    }
  }, [])

  async function logout() {
    await fetch('/auth/logout', { method: 'POST' })
    setState({ status: 'anonymous' })
  }

  function accountDeleted() {
    setState({ status: 'anonymous' })
  }

  return (
    <AuthContext.Provider value={{ ...state, logout, accountDeleted }}>
      {children}
    </AuthContext.Provider>
  )
}
