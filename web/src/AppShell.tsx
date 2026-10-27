import { Outlet } from 'react-router'
import { useAuth } from './useAuth'

export function AppShell() {
  const auth = useAuth()
  if (auth.status !== 'user') return null

  return (
    <>
      <header>
        <span>{auth.user.firstName}</span>
        <button type="button" onClick={() => void auth.logout()}>
          Cerrar sesión
        </button>
      </header>
      <main>
        <Outlet />
      </main>
    </>
  )
}
