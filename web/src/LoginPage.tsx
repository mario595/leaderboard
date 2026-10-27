import { Navigate } from 'react-router'
import { useAuth } from './useAuth'

export function LoginPage() {
  const auth = useAuth()
  if (auth.status === 'loading') return null
  if (auth.status === 'user') return <Navigate to="/" replace />
  return <a href="/auth/strava">Conectar con Strava</a>
}
