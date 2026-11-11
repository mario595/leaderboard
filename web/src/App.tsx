import { Navigate, Route, Routes } from 'react-router'
import { AccountDeletedPage } from './pages/AccountDeletedPage'
import { ActivitiesPage } from './pages/ActivitiesPage'
import { AppShell } from './components/AppShell'
import { LeaderboardPage } from './pages/LeaderboardPage'
import { JoinPage } from './pages/JoinPage'
import { LoginPage } from './pages/LoginPage'
import { PrivacyPage } from './pages/PrivacyPage'
import { PublicLayout } from './components/PublicLayout'
import { RulesPage } from './pages/RulesPage'
import { TermsPage } from './pages/TermsPage'
import { RequireAuth } from './auth/RequireAuth'

// AppRoutes is the route table. /login, /join (the consent step of an
// invitation), /privacy, /terms and /account-deleted are public (inside
// PublicLayout); every other page sits behind RequireAuth
// (logged-in users only) inside AppShell (top bar and tab bar).
export function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/join" element={<JoinPage />} />
      <Route element={<PublicLayout />}>
        <Route path="privacy" element={<PrivacyPage />} />
        <Route path="terms" element={<TermsPage />} />
        <Route path="account-deleted" element={<AccountDeletedPage />} />
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
