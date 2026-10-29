import type { Standing } from './api'
import { initials } from './format'

// Medal colours for the podium, by rank: members tied at 2 both get silver.
const medals: Record<number, string> = { 1: 'gold', 2: 'silver', 3: 'bronze' }

// StandingsRow is one member of the ranking: rank (with a medal on the
// podium), initials, name, a "Tú" badge on the user's own row, and points.
// The rank comes from the API, never computed here.
export function StandingsRow({
  standing,
  isMe,
}: {
  standing: Standing
  isMe: boolean
}) {
  const rowClass = [
    'standings__row',
    isMe && 'standings__row--me',
    standing.points === 0 && 'standings__row--zero',
  ]
    .filter(Boolean)
    .join(' ')
  const medal = medals[standing.rank]
  const rankClass = medal ? `rank rank--${medal}` : 'rank'

  return (
    <li className={rowClass}>
      <span className={rankClass}>
        <span className="visually-hidden">Puesto </span>
        {standing.rank}
      </span>
      <span className="avatar" aria-hidden="true">
        {initials(standing.firstName, standing.lastName)}
      </span>
      <span className="standings__member">
        <span className="standings__name">
          {standing.firstName} {standing.lastName}
        </span>
        {isMe && <span className="badge badge--you">Tú</span>}
      </span>
      <span className="standings__points">
        <span className="standings__points-value">{standing.points}</span>
        <abbr className="standings__points-unit" title="puntos">
          pts
        </abbr>
      </span>
    </li>
  )
}
