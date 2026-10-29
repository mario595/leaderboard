import type { ComponentType } from 'react'
import type { Activity } from '../api/api'
import {
  formatDistance,
  formatDuration,
  formatTime,
  sport,
  type SportKind,
} from '../api/format'
import { ActivityIcon, RideIcon, RunIcon, SwimIcon, TrophyIcon } from './icons'

// Why an activity scored 0 points, by the API's zeroReason code. The rules
// live in Go; the page only words them.
const reasons: Record<string, string> = {
  sport: 'Deporte no puntuable',
  short: 'Menos de 30 min',
  daily_cap: 'Límite diario alcanzado',
}

const icons: Record<SportKind, ComponentType> = {
  run: RunIcon,
  ride: RideIcon,
  swim: SwimIcon,
  other: ActivityIcon,
}

// ActivityCard is one activity: sport icon and label, start time, the
// competition badge, moving time, distance and points. A 0-point activity is
// de-emphasised and says why it scored nothing.
export function ActivityCard({ activity }: { activity: Activity }) {
  const { label, kind } = sport(activity.sportType)
  const Icon = icons[kind]
  const zero = activity.points === 0
  const reason = reasons[activity.zeroReason]

  return (
    <li
      className={zero ? 'activity-card activity-card--zero' : 'activity-card'}
    >
      <span className="activity-card__icon">
        <Icon />
      </span>
      <div className="activity-card__body">
        <p className="activity-card__title">
          <span className="activity-card__sport">{label}</span>
          <span className="activity-card__time">
            <time dateTime={activity.startLocal}>
              {formatTime(activity.startLocal)}
            </time>
          </span>
          {activity.competition && (
            <span className="badge badge--competition">
              <TrophyIcon small />
              Competición
            </span>
          )}
        </p>
        <p className="activity-card__meta">
          <span>
            <span className="visually-hidden">Tiempo en movimiento: </span>
            {formatDuration(activity.movingTime)}
          </span>
          <span aria-hidden="true">·</span>
          <span>
            <span className="visually-hidden">Distancia: </span>
            {formatDistance(activity.distance, activity.sportType)}
          </span>
        </p>
        {reason && <p className="activity-card__reason">{reason}</p>}
      </div>
      <p className="activity-card__points">
        <span className="activity-card__points-value">{activity.points}</span>
        <abbr className="activity-card__points-unit" title="puntos">
          pts
        </abbr>
      </p>
    </li>
  )
}
