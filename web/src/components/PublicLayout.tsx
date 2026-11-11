import { Link, Outlet } from 'react-router'
import { BrandIcon } from './icons'
import { SiteFooter } from './SiteFooter'

// PublicLayout frames the pages anyone can open (privacy policy, terms):
// the brand, linking home, the page and the footer. It does not depend on
// the session, so the pages show even while it is being checked.
export function PublicLayout() {
  return (
    <div className="app">
      <div className="app__shell">
        <header className="top-bar">
          <div className="top-bar__inner">
            <Link className="top-bar__brand" to="/">
              <BrandIcon />
              <span>Leaderboard</span>
            </Link>
          </div>
        </header>
        <main className="app__main">
          <Outlet />
        </main>
        <SiteFooter />
      </div>
    </div>
  )
}
