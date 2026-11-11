import { createContext, useContext } from 'react'
import { type Me } from '../api/api'

export type AuthState =
  | { status: 'loading' }
  | { status: 'anonymous' }
  | { status: 'error' }
  | { status: 'user'; user: Me }

// logout ends the session on the server; accountDeleted only forgets the
// user, after DELETE /api/me has already ended every session.
export type Auth = AuthState & {
  logout: () => Promise<void>
  accountDeleted: () => void
}

export const AuthContext = createContext<Auth | null>(null)

export function useAuth(): Auth {
  const auth = useContext(AuthContext)
  if (auth === null) {
    throw new Error('useAuth must be inside <AuthProvider>')
  }
  return auth
}
