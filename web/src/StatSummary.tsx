// StatSummary shows the user's rank and points side by side. rank is null
// when the user is not in the standings (cannot happen today: every member
// is listed, with 0 points if need be).
export function StatSummary({
  rank,
  points,
}: {
  rank: number | null
  points: number
}) {
  return (
    <dl className="stat-summary">
      <div className="stat stat--accent">
        <dt className="stat__label">Tu posición</dt>
        <dd className="stat__value">{rank === null ? '–' : `${rank}º`}</dd>
      </div>
      <div className="stat">
        <dt className="stat__label">Tus puntos</dt>
        <dd className="stat__value">
          {points}
          <span className="stat__unit">pts</span>
        </dd>
      </div>
    </dl>
  )
}
