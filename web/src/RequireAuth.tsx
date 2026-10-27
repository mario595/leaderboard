import { Navigate, Outlet } from 'react-router'
import { AlertIcon } from './icons'
import { useAuth } from './useAuth'

// RequireAuth guards every page below it: nothing while the session is being
// checked (no flash of the login page), an error when the check failed (a
// server error is not a logout), the login page for anonymous visitors, and
// the page itself for a logged-in user.
export function RequireAuth() {
  const auth = useAuth()
  switch (auth.status) {
    case 'loading':
      return null
    case 'error':
      return (
        <div className="app">
          <main className="login">
            <div className="login__card">
              <div className="alert" role="alert">
                <AlertIcon />
                <p className="alert__text">
                  No hemos podido comprobar tu sesión. Recarga la página en unos
                  minutos.
                </p>
              </div>
            </div>
          </main>
        </div>
      )
    case 'anonymous':
      return <Navigate to="/login" replace />
    case 'user':
      return <Outlet />
  }
}
