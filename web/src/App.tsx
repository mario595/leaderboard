import { Route, Routes } from 'react-router'
import { AppShell } from './AppShell'
import { LoginPage } from './LoginPage'
import { PageHeader } from './PageHeader'
import { RequireAuth } from './RequireAuth'

// AppRoutes is the route table. /login is public; every other page sits
// behind RequireAuth (logged-in users only) inside AppShell (top bar and tab
// bar). The pages are placeholders until #19 and #20.
export function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route element={<RequireAuth />}>
        <Route element={<AppShell />}>
          <Route index element={<PageHeader title="Clasificación" />} />
          <Route
            path="activities"
            element={<PageHeader title="Mis actividades" />}
          />
          <Route path="rules" element={<PageHeader title="Cómo se puntúa" />} />
        </Route>
      </Route>
    </Routes>
  )
}
