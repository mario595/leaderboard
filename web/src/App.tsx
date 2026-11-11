import { Navigate, Route, Routes } from 'react-router'
import { ActivitiesPage } from './pages/ActivitiesPage'
import { AppShell } from './components/AppShell'
import { LeaderboardPage } from './pages/LeaderboardPage'
import { LoginPage } from './pages/LoginPage'
import { PrivacyPage } from './pages/PrivacyPage'
import { PublicLayout } from './components/PublicLayout'
import { RulesPage } from './pages/RulesPage'
import { TermsPage } from './pages/TermsPage'
import { RequireAuth } from './auth/RequireAuth'

// AppRoutes is the route table. /login, /privacy and /terms are public (the
// legal pages inside PublicLayout); every other page sits behind RequireAuth
// (logged-in users only) inside AppShell (top bar and tab bar).
export function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route element={<PublicLayout />}>
        <Route path="privacy" element={<PrivacyPage />} />
        <Route path="terms" element={<TermsPage />} />
      </Route>
      <Route element={<RequireAuth />}>
        <Route element={<AppShell />}>
          <Route index element={<LeaderboardPage />} />
          <Route path="activities" element={<ActivitiesPage />} />
          <Route path="rules" element={<RulesPage />} />
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  )
}
