import { Navigate, useSearchParams } from 'react-router'
import connectButton from '../assets/strava-connect.svg'
import poweredByBlack from '../assets/strava-powered-by-black.svg'
import poweredByWhite from '../assets/strava-powered-by-white.svg'
import { AlertIcon, BrandIcon } from '../components/icons'
import { useAuth } from '../auth/useAuth'

// Messages for the reasons the OAuth callback adds as ?error= (see
// internal/auth/callback.go). An unknown reason gets the generic one.
const loginErrors: Record<string, string> = {
  cancelled: 'Has cancelado el inicio de sesión en Strava.',
  scope:
    'Necesitamos permiso para ver tus actividades. Vuelve a conectar y deja marcada la casilla de actividades.',
  failed: 'No se pudo iniciar sesión. Inténtalo de nuevo.',
  invitation:
    'Necesitas una invitación para unirte. Pide el enlace a tu grupo.',
}

// LoginPage is the only public page. The Strava button is a plain link to the
// Go server: the OAuth flow needs a full page load to Strava and back, not a
// client-side route. A logged-in user is sent to the leaderboard.
export function LoginPage() {
  const auth = useAuth()
  const [params] = useSearchParams()

  if (auth.status === 'loading') return null
  if (auth.status === 'user') return <Navigate to="/" replace />

  const errorCode = params.get('error')
  const message =
    errorCode === null
      ? null
      : (loginErrors[errorCode] ?? loginErrors['failed'])

  return (
    <div className="app">
      <main className="login">
        <div className="login__card">
          {message && (
            <div className="alert" role="alert">
              <AlertIcon />
              <p className="alert__text">{message}</p>
            </div>
          )}
          <h1 className="login__brand">
            <BrandIcon />
            <span>Leaderboard</span>
          </h1>
          <p className="login__pitch">
            Entrena. Suma puntos.{' '}
            <span className="login__pitch-accent">
              Sube en la clasificación.
            </span>
          </p>
          <ul className="login__rules" aria-label="Cómo se puntúa, en resumen">
            <li className="login__rule">
              <span className="login__rule-value" aria-hidden="true">
                2
              </span>
              <span>
                <strong>2 puntos</strong> por carrera, bici o natación de 30 min
                o más.
              </span>
            </li>
            <li className="login__rule">
              <span className="login__rule-value" aria-hidden="true">
                4
              </span>
              <span>
                <strong>4 puntos</strong> si es una competición.
              </span>
            </li>
            <li className="login__rule">
              <span className="login__rule-value" aria-hidden="true">
                ×2
              </span>
              <span>
                Cuentan <strong>hasta 2 actividades al día</strong>, las de más
                puntos.
              </span>
            </li>
          </ul>
          <a className="strava-button" href="/auth/strava">
            <img
              src={connectButton}
              alt="Connect with Strava"
              width={237}
              height={48}
            />
          </a>
          <p className="login__hint">
            Entras o te registras con tu cuenta de Strava. Tus actividades
            llegan solas.
          </p>
        </div>
        <footer className="login__footer">
          <picture>
            <source
              srcSet={poweredByWhite}
              media="(prefers-color-scheme: dark)"
            />
            <img src={poweredByBlack} alt="Powered by Strava" />
          </picture>
        </footer>
      </main>
    </div>
  )
}
