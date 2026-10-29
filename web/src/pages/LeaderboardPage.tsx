import { Link } from 'react-router'
import type { Leaderboard } from '../api/api'
import { EmptyState } from '../components/EmptyState'
import { formatSeasonRange } from '../api/format'
import { AlertIcon, CalendarIcon } from '../components/icons'
import { PageHeader } from '../components/PageHeader'
import { StandingsRow } from '../components/StandingsRow'
import { StatSummary } from '../components/StatSummary'
import { useApi } from '../api/useApi'
import { useAuth } from '../auth/useAuth'

// LeaderboardPage shows the current season's ranking: the user's rank and
// points, then every member in the API's order (the API ranks; ties share a
// rank). States: loading (skeleton), no season (404), error (with retry),
// ready.
export function LeaderboardPage() {
  // Hooks first: React needs the same hooks in the same order on every render.
  const board = useApi<Leaderboard>('/api/leaderboard')
  const auth = useAuth()

  if (board.status === 'loading') {
    return <LeaderboardSkeleton />
  }

  if (board.status === 'error') {
    if (board.httpStatus === 404) {
      return (
        <>
          <PageHeader title="Clasificación" />
          <EmptyState
            icon={<CalendarIcon />}
            title="Ahora mismo no hay ninguna temporada en curso."
            text="Cuando empiece la próxima, verás aquí la clasificación."
          />
        </>
      )
    }
    return (
      <>
        <PageHeader title="Clasificación" />
        <EmptyState
          icon={<AlertIcon />}
          title="Algo ha fallado."
          text="No hemos podido cargar la clasificación."
          isError
        >
          <button
            className="button button--primary"
            type="button"
            onClick={board.retry}
          >
            Reintentar
          </button>
        </EmptyState>
      </>
    )
  }

  const { season, standings } = board.data
  const myId = auth.status === 'user' ? auth.user.id : null
  const mine = standings.find((s) => s.userId === myId)

  return (
    <>
      <PageHeader
        title="Clasificación"
        eyebrow={`${season.name} · ${formatSeasonRange(season.startDate, season.endDate)}`}
      />
      <StatSummary rank={mine?.rank ?? null} points={mine?.points ?? 0} />
      <ol className="standings" aria-label="Clasificación de la temporada">
        {standings.map((s) => (
          <StandingsRow key={s.userId} standing={s} isMe={s.userId === myId} />
        ))}
      </ol>
      <p className="standings__footer">
        <Link to="/rules">¿Cómo se puntúa?</Link>
      </p>
    </>
  )
}

// LeaderboardSkeleton mirrors the page's layout while the standings load,
// so nothing jumps when they arrive. Screen readers hear the status text.
function LeaderboardSkeleton() {
  return (
    <div aria-busy="true">
      <p className="visually-hidden" role="status">
        Cargando la clasificación…
      </p>
      <div className="page-header" aria-hidden="true">
        <span className="skeleton skeleton--short" />
        <span className="skeleton skeleton--title" />
      </div>
      <div className="stat-summary" aria-hidden="true">
        <span className="skeleton skeleton--stat" />
        <span className="skeleton skeleton--stat" />
      </div>
      <ul className="standings" aria-hidden="true">
        {Array.from({ length: 6 }, (_, i) => (
          <li className="standings__row" key={i}>
            <span className="skeleton skeleton--circle" />
            <span className="skeleton skeleton--avatar" />
            <span className="skeleton skeleton--line" />
            <span className="skeleton skeleton--number" />
          </li>
        ))}
      </ul>
    </div>
  )
}
