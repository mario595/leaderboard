import { Route, Routes } from 'react-router'
import { ActivitiesPage } from './pages/ActivitiesPage'
import { AppShell } from './components/AppShell'
import { LeaderboardPage } from './pages/LeaderboardPage'
import { LoginPage } from './pages/LoginPage'
import { PageHeader } from './components/PageHeader'
import { RequireAuth } from './auth/RequireAuth'

// AppRoutes is the route table. /login is public; every other page sits
// behind RequireAuth (logged-in users only) inside AppShell (top bar and tab
// bar). The activities and rules pages are placeholders until #20.
export function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route element={<RequireAuth />}>
        <Route element={<AppShell />}>
          <Route index element={<LeaderboardPage />} />
          <Route path="activities" element={<ActivitiesPage />} />
          <Route path="rules" element={<PageHeader title="Cómo se puntúa" />} />
        </Route>
      </Route>
    </Routes>
  )
}
