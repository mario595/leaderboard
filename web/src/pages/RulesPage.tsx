import type { ReactNode } from 'react'
import type { Leaderboard } from '../api/api'
import { formatSeasonRange, formatShortDate } from '../api/format'
import { useApi } from '../api/useApi'
import {
  ActivityIcon,
  CalendarIcon,
  RefreshIcon,
  StopwatchIcon,
  TrophyIcon,
} from '../components/icons'
import { PageHeader } from '../components/PageHeader'

// The sports that score, as labelled on the activity cards. The rule itself
// lives in Go (scoring.DisciplineOf); keep this list in sync with it.
const countedSports = [
  'Carrera',
  'Trail',
  'Carrera virtual',
  'Ciclismo',
  'BTT',
  'Gravel',
  'Ciclismo virtual',
  'Natación',
]

// RulesPage explains how points are scored. The text is static; only the
// season (from the leaderboard) is loaded, and left out while loading or
// when there is no season.
export function RulesPage() {
  const board = useApi<Leaderboard>('/api/leaderboard')
  const season = board.status === 'ready' ? board.data.season : null

  return (
    <>
      <PageHeader
        title="Cómo se puntúa"
        eyebrow={
          season
            ? `${season.name} · ${formatSeasonRange(season.startDate, season.endDate)}`
            : undefined
        }
      />
      <ol className="rules">
        <RuleCard icon={<ActivityIcon />} title="Deportes que cuentan">
          <ul className="chip-list" aria-label="Deportes que cuentan">
            {countedSports.map((s) => (
              <li className="chip" key={s}>
                {s}
              </li>
            ))}
          </ul>
          <p className="rule-card__text">
            Las bicis eléctricas y el resto de deportes no suman.
          </p>
        </RuleCard>

        <RuleCard icon={<StopwatchIcon />} title="Al menos 30 minutos">
          <p className="rule-card__text">
            Cuenta el <strong>tiempo en movimiento</strong> de Strava, no el
            tiempo total: las paradas no suman.
          </p>
        </RuleCard>

        <RuleCard
          icon={<span aria-hidden="true">+</span>}
          title="2 puntos, o 4 si compites"
        >
          <div className="points-pair">
            <p className="points-pair__item">
              <span className="points-pair__value">2</span>
              <span className="points-pair__label">por actividad</span>
            </p>
            <p className="points-pair__item">
              <span className="points-pair__value">4</span>
              <span className="points-pair__label">si es competición</span>
            </p>
          </div>
        </RuleCard>

        <RuleCard icon={<TrophyIcon />} title="¿Qué es una competición?">
          <p className="rule-card__text">
            Una carrera a pie o en bici marcada en Strava con el tipo de
            entrenamiento <strong>«Carrera»</strong> (Race).
          </p>
          <p className="rule-card__text">
            O escribe <span className="code">#competition</span> en la
            descripción de la actividad. Vale para cualquier deporte, también
            natación.
          </p>
        </RuleCard>

        <RuleCard
          icon={<span aria-hidden="true">×2</span>}
          title="Máximo 2 actividades al día"
        >
          <p className="rule-card__text">
            Cada día puntúan como mucho dos actividades: las que más puntos dan.
            Las demás de ese día se quedan en 0.
          </p>
          <p className="rule-card__text">
            El día es el de la hora de inicio de la actividad, en tu hora local.
          </p>
        </RuleCard>

        <RuleCard icon={<CalendarIcon />} title="Solo dentro de la temporada">
          <p className="rule-card__text">
            Cuentan las actividades entre el inicio y el fin de la temporada.
            {season && (
              <>
                {' '}
                Ahora:{' '}
                <strong>
                  {season.name}, del {formatShortDate(season.startDate)} al{' '}
                  {formatShortDate(season.endDate)}
                </strong>
                .
              </>
            )}
          </p>
        </RuleCard>

        <RuleCard icon={<RefreshIcon />} title="¿Cambiaste algo en Strava?">
          <p className="rule-card__text">
            Si editas una actividad (por ejemplo, añades{' '}
            <span className="code">#competition</span>), los puntos se
            recalculan solos. Puede tardar hasta un par de horas.
          </p>
        </RuleCard>
      </ol>
    </>
  )
}

// RuleCard is one rule: an icon (or a short symbol), a title and its text.
function RuleCard({
  icon,
  title,
  children,
}: {
  icon: ReactNode
  title: string
  children: ReactNode
}) {
  return (
    <li className="rule-card">
      <span className="rule-card__icon">{icon}</span>
      <div className="rule-card__body">
        <h2 className="rule-card__title">{title}</h2>
        {children}
      </div>
    </li>
  )
}
