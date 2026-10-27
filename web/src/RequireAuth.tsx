import { useAuth } from './useAuth'
import { Navigate, Outlet } from 'react-router'

export function RequireAuth() {
  const auth = useAuth()
  switch (auth.status) {
    case 'loading':
      return null
    case 'error':
      return (
        <p role="alert">
          No hemos podido comprobar tu sesión. Recarga la página.
        </p>
      )
    case 'anonymous':
      return <Navigate to="/login" replace />
    case 'user':
      return <Outlet />
  }
}
