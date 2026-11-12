import { Link, useLocation } from 'react-router'
import type { AccountDeletion } from '../api/api'
import { formatDateTime, formatLongDate } from '../api/format'
import { PageHeader } from '../components/PageHeader'
import { supportEmail } from '../legal'

// AccountDeletedPage is the written confirmation of a deletion (Strava API
// Policy 2.5): the app has no email address, so this page is it. The
// deletion's time and backup date come in the router state from
// AccountSection; opened without them (a reload, a shared link) it shows the
// same text without the dates.
export function AccountDeletedPage() {
  const deletion = (useLocation().state as AccountDeletion | null) ?? null

  return (
    <>
      <PageHeader title="Tus datos se han borrado" />
      <article className="legal">
        <p>
          {deletion
            ? `El ${formatDateTime(deletion.deletedAt)} borramos tu cuenta de Leaderboard:`
            : 'Hemos borrado tu cuenta de Leaderboard:'}
        </p>
        <ul>
          <li>tu nombre, tu identificador de Strava y tus tokens de acceso;</li>
          <li>todas tus actividades y tus puntos;</li>
          <li>los avisos de Strava pendientes y tus sesiones abiertas.</li>
        </ul>
        <p>
          También hemos quitado el acceso de la aplicación a tu cuenta de
          Strava. Tus actividades en Strava no se han tocado.
        </p>
        <p>
          {deletion
            ? `Las copias de seguridad que aún los contengan desaparecen como muy tarde el ${formatLongDate(deletion.backupsGoneBy)}.`
            : 'Las copias de seguridad que aún los contengan desaparecen como muy tarde 30 días después del borrado.'}
        </p>
        <p>
          Esta página es tu confirmación por escrito. Si necesitas algo más,
          escribe a <a href={`mailto:${supportEmail}`}>{supportEmail}</a>.
        </p>
        <p>
          <Link to="/login">Volver al inicio</Link>
        </p>
      </article>
    </>
  )
}
