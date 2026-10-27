import { createContext, useContext } from 'react'
import { type Me } from './api'

export type AuthState =
  | { status: 'loading' }
  | { status: 'anonymous' }
  | { status: 'error' }
  | { status: 'user'; user: Me }

export type Auth = AuthState & { logout: () => Promise<void> }

export const AuthContext = createContext<Auth | null>(null)

export function useAuth(): Auth {
  const auth = useContext(AuthContext)
  if (auth === null) {
    throw new Error('useAuth must be inside <AuthProvider>')
  }
  return auth
}
