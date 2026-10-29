import { NavLink, Outlet } from 'react-router'
import {
  ActivityIcon,
  BrandIcon,
  InfoIcon,
  LogoutIcon,
  StandingsIcon,
} from './icons'
import { useAuth } from '../auth/useAuth'

// AppShell is the frame around every logged-in page: the top bar (brand,
// links on wide screens, the user and logout), the page, and the tab bar on
// phones. NavLink marks the current page with aria-current="page", which the
// styles highlight.
export function AppShell() {
  const auth = useAuth()
  // RequireAuth renders this only for a user; the check narrows the type.
  if (auth.status !== 'user') return null

  return (
    <div className="app">
      <div className="app__shell">
        <header className="top-bar">
          <div className="top-bar__inner">
            <NavLink className="top-bar__brand" to="/" end>
              <BrandIcon />
              <span>Leaderboard</span>
            </NavLink>
            <nav className="top-bar__nav" aria-label="Principal">
              <NavLink className="nav-link" to="/" end>
                Clasificación
              </NavLink>
              <NavLink className="nav-link" to="/activities">
                Mis actividades
              </NavLink>
              <NavLink className="nav-link" to="/rules">
                Cómo se puntúa
              </NavLink>
            </nav>
            <div className="top-bar__user">
              <span className="top-bar__name">{auth.user.firstName}</span>
              <button
                className="button button--ghost button--small"
                type="button"
                onClick={() => void auth.logout()}
              >
                <LogoutIcon />
                <span className="top-bar__logout-label">Cerrar sesión</span>
              </button>
            </div>
          </div>
        </header>

        <main className="app__main">
          <Outlet />
        </main>

        <nav className="tab-bar" aria-label="Principal (móvil)">
          <NavLink className="tab-bar__link" to="/" end>
            <StandingsIcon />
            <span>Clasificación</span>
          </NavLink>
          <NavLink className="tab-bar__link" to="/activities">
            <ActivityIcon />
            <span>Mis actividades</span>
          </NavLink>
          <NavLink className="tab-bar__link" to="/rules">
            <InfoIcon />
            <span>Reglas</span>
          </NavLink>
        </nav>
      </div>
    </div>
  )
}
