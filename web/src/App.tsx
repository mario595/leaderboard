import { Route, Routes } from 'react-router'
import { AppShell } from './AppShell'
import { LoginPage } from './LoginPage'
import { RequireAuth } from './RequireAuth'

function App() {
  return <h1>Leaderboard</h1>
}

export default App

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route element={<RequireAuth />}>
        <Route element={<AppShell />}>
          <Route index element={<h1>Clasificación</h1>} />
          <Route path="activities" element={<h1>Mis actividades</h1>} />
          <Route path="rules" element={<h1>Cómo se puntúa</h1>} />
        </Route>
      </Route>
    </Routes>
  )
}
