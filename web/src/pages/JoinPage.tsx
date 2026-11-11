import { useState } from 'react'
import { Link, Navigate } from 'react-router'
import connectButton from '../assets/strava-connect.svg'
import poweredByBlack from '../assets/strava-powered-by-black.svg'
import poweredByWhite from '../assets/strava-powered-by-white.svg'
import { BrandIcon } from '../components/icons'
import { SiteFooter } from '../components/SiteFooter'
import { useAuth } from '../auth/useAuth'
import { supportEmail } from '../legal'

// JoinPage is the consent step of an invitation: /invite/{code} keeps the
// code in the session and lands here. It says what the app reads, how, who
// sees it, how to withdraw and how to ask for deletion (Strava API Policy
// 2.1), and asks the athlete to agree that the group sees their name and
// points (5.13). The Strava button is a link only once they agree; until
// then a hint asks them to tick the box. Consent
// is not stored. A logged-in member is sent to the leaderboard.
export function JoinPage() {
  const auth = useAuth()
  const [agreed, setAgreed] = useState(false)

  if (auth.status === 'loading') return null
  if (auth.status === 'user') return <Navigate to="/" replace />

  const button = (
    <img
      src={connectButton}
      alt="Connect with Strava"
      width={237}
      height={48}
    />
  )

  return (
    <div className="app">
      <main className="login">
        <div className="login__card">
          <h1 className="login__brand">
            <BrandIcon />
            <span>Leaderboard</span>
          </h1>
          <h2 className="consent__title">Antes de conectar tu Strava</h2>

          <ul className="consent__list">
            <li>
              <strong>Qué leemos:</strong> tu nombre, tu identificador de Strava
              y, de cada actividad, el deporte, la fecha y hora, el tiempo en
              movimiento, la distancia, el tipo de entrenamiento y la
              descripción (solo para buscar{' '}
              <span className="code">#competition</span>; nunca se muestra).
            </li>
            <li>
              <strong>Cómo:</strong> Strava nos lo da al conectar tu cuenta y
              cada vez que creas, cambias o borras una actividad.
            </li>
            <li>
              <strong>Quién lo ve:</strong> los miembros del grupo ven tu nombre
              y tus puntos. Tus actividades solo las ves tú.
            </li>
            <li>
              <strong>Retirar tu consentimiento:</strong> cuando quieras, desde
              «Mis actividades» o retirando el acceso a la aplicación en Strava
              (Ajustes → Mis aplicaciones). Al hacerlo borramos todos tus datos.
            </li>
            <li>
              <strong>Pedir el borrado:</strong> con el botón «Desconectar y
              borrar mis datos» de «Mis actividades», o escribiendo a{' '}
              <a href={`mailto:${supportEmail}`}>{supportEmail}</a>. Te
              confirmaremos el borrado cuando esté hecho: en la aplicación al
              momento, o por email.
            </li>
          </ul>

          <label className="consent__check">
            <input
              type="checkbox"
              checked={agreed}
              onChange={(e) => setAgreed(e.target.checked)}
            />
            <span>
              Acepto la <Link to="/privacy">política de privacidad</Link> y que
              mi nombre y mis puntos se muestren a los miembros del grupo.
            </span>
          </label>

          {agreed ? (
            <a className="strava-button" href="/auth/strava">
              {button}
            </a>
          ) : (
            // Not a link until the athlete agrees: nothing to click through.
            // The image stays unmodified (Strava's guidelines); the hint
            // below says to tick the box.
            <span
              className="strava-button strava-button--disabled"
              aria-disabled="true"
            >
              {button}
            </span>
          )}
          <p className="login__hint">
            {agreed
              ? 'Te llevamos a Strava para que autorices el acceso.'
              : 'Marca la casilla para continuar.'}
          </p>
        </div>
        {/* A div, not a footer: SiteFooter is the footer. */}
        <div className="login__footer">
          <picture>
            <source
              srcSet={poweredByWhite}
              media="(prefers-color-scheme: dark)"
            />
            <img src={poweredByBlack} alt="Powered by Strava" />
          </picture>
          <SiteFooter />
        </div>
      </main>
    </div>
  )
}
