import { Link } from 'react-router'
import type { Leaderboard } from '../api/api'
import { formatSeasonRange, groupByDay } from '../api/format'
import { useActivities } from '../api/useActivities'
import { useApi } from '../api/useApi'
import { useAuth } from '../auth/useAuth'
import { AccountSection } from '../components/AccountSection'
import { ActivityDay } from '../components/ActivityDay'
import { EmptyState } from '../components/EmptyState'
import { AlertIcon, CalendarIcon, RunIcon } from '../components/icons'
import { PageHeader } from '../components/PageHeader'
import { StatSummary } from '../components/StatSummary'

// ActivitiesPage shows my activities of the current season, grouped by day,
// newest first, with "Cargar más" while there are older ones. My rank and
// points come from a second request (the leaderboard), loaded in parallel; the
// activities show even if that one fails. States: loading (skeleton), no
// season (404), error (with retry), empty, ready. Below the feed, in every
// state, the member's account section (leave and delete my data).
export function ActivitiesPage() {
  return (
    <>
      <ActivitiesFeed />
      <AccountSection />
    </>
  )
}

function ActivitiesFeed() {
  // Hooks first: React needs the same hooks in the same order on every render.
  const feed = useActivities()
  const board = useApi<Leaderboard>('/api/leaderboard')
  const auth = useAuth()

  if (feed.status === 'loading') {
    return <ActivitiesSkeleton />
  }

  if (feed.status === 'error') {
    if (feed.httpStatus === 404) {
      return (
        <>
          <PageHeader title="Mis actividades" />
          <EmptyState
            icon={<CalendarIcon />}
            title="Ahora mismo no hay ninguna temporada en curso."
            text="Cuando empiece la próxima, verás aquí tus actividades."
          />
        </>
      )
    }
    return (
      <>
        <PageHeader title="Mis actividades" />
        <EmptyState
          icon={<AlertIcon />}
          title="Algo ha fallado."
          text="No hemos podido cargar tus actividades."
          isError
        >
          <button
            className="button button--primary"
            type="button"
            onClick={feed.retry}
          >
            Reintentar
          </button>
        </EmptyState>
      </>
    )
  }

  const me = auth.status === 'user' ? auth.user : null
  const mine =
    board.status === 'ready'
      ? board.data.standings.find((s) => s.userId === me?.id)
      : undefined
  const header = (
    <PageHeader
      title={me ? `${me.firstName} ${me.lastName}` : 'Mis actividades'}
      eyebrow={`${feed.season.name} · ${formatSeasonRange(feed.season.startDate, feed.season.endDate)}`}
    />
  )

  if (feed.activities.length === 0) {
    return (
      <>
        {header}
        <EmptyState
          icon={<RunIcon />}
          title="Aún no tienes actividades esta temporada. ¡A entrenar!"
          text="Tus actividades de Strava aparecerán aquí poco después de subirlas."
        >
          <Link className="button button--secondary" to="/rules">
            Ver cómo se puntúa
          </Link>
        </EmptyState>
      </>
    )
  }

  return (
    <>
      {header}
      {board.status === 'loading' && <StatsSkeleton />}
      {board.status === 'ready' && (
        <StatSummary rank={mine?.rank ?? null} points={mine?.points ?? 0} />
      )}
      <p className="activity-feed__note">
        Si editas una actividad en Strava, sus puntos pueden tardar hasta un par
        de horas en actualizarse.
      </p>
      {groupByDay(feed.activities).map((day) => (
        <ActivityDay key={day.date} day={day} />
      ))}
      {feed.hasMore && (
        <div className="load-more">
          {feed.loadMoreFailed && (
            <p className="load-more__error" role="alert">
              No hemos podido cargar más actividades.
            </p>
          )}
          <button
            className="button button--secondary"
            type="button"
            onClick={feed.loadMore}
            disabled={feed.loadingMore}
          >
            {feed.loadingMore ? 'Cargando…' : 'Cargar más'}
          </button>
        </div>
      )}
    </>
  )
}

function StatsSkeleton() {
  return (
    <div className="stat-summary" aria-hidden="true">
      <span className="skeleton skeleton--stat" />
      <span className="skeleton skeleton--stat" />
    </div>
  )
}

// ActivitiesSkeleton mirrors the page's layout while the first page loads, so
// nothing jumps when it arrives. Screen readers hear the status text.
function ActivitiesSkeleton() {
  return (
    <div aria-busy="true">
      <p className="visually-hidden" role="status">
        Cargando tus actividades…
      </p>
      <div className="page-header" aria-hidden="true">
        <span className="skeleton skeleton--short" />
        <span className="skeleton skeleton--title" />
      </div>
      <StatsSkeleton />
      <ul className="activity-day__list" aria-hidden="true">
        {Array.from({ length: 5 }, (_, i) => (
          <li className="skeleton-card" key={i}>
            <span className="skeleton skeleton--avatar" />
            <span className="skeleton-card__lines">
              <span className="skeleton skeleton--line" />
              <span className="skeleton skeleton--short" />
            </span>
            <span className="skeleton skeleton--number" />
          </li>
        ))}
      </ul>
    </div>
  )
}
