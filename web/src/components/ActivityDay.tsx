import type { Activity } from '../api/api'
import { formatDay, type Day } from '../api/format'
import { ActivityCard } from './ActivityCard'

// ActivityDay is one day of the activity list: the date and the day's points
// (which explain a "daily cap" card at a glance), then its activities.
export function ActivityDay({ day }: { day: Day<Activity> }) {
  return (
    <section className="activity-day">
      <h2 className="activity-day__header">
        <span className="activity-day__date">
          <time dateTime={day.date}>{formatDay(day.date)}</time>
        </span>
        <span className="activity-day__points">{day.points} pts</span>
      </h2>
      <ul className="activity-day__list">
        {day.activities.map((a) => (
          <ActivityCard key={a.id} activity={a} />
        ))}
      </ul>
    </section>
  )
}
