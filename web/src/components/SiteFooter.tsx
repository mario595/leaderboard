import { Link } from 'react-router'
import { useAuth } from '../auth/useAuth'
import { stravaAthleteURL, supportEmail } from '../legal'

// SiteFooter is on every page: links to the privacy policy and the terms,
// the support contact, and, for a logged-in member, their Strava account. It
// never waits for the session check: until it answers, the account link is
// simply left out.
export function SiteFooter() {
  const auth = useAuth()

  return (
    <footer className="site-footer">
      <nav className="site-footer__nav" aria-label="Pie de página">
        <Link className="site-footer__link" to="/privacy">
          Privacidad
        </Link>
        <Link className="site-footer__link" to="/terms">
          Términos
        </Link>
        <a className="site-footer__link" href={`mailto:${supportEmail}`}>
          Soporte
        </a>
        {auth.status === 'user' && (
          <a
            className="site-footer__link"
            href={stravaAthleteURL(auth.user.stravaId)}
            target="_blank"
            rel="noreferrer"
          >
            Tu cuenta de Strava
          </a>
        )}
      </nav>
    </footer>
  )
}
