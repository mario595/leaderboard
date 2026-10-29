// Spanish month abbreviations, as in the design.
const MONTHS = [
  'ene',
  'feb',
  'mar',
  'abr',
  'may',
  'jun',
  'jul',
  'ago',
  'sept',
  'oct',
  'nov',
  'dic',
]

// Spanish weekday abbreviations, Sunday first (as Date.getDay counts).
const WEEKDAY = ['dom', 'lun', 'mar', 'mié', 'jue', 'vie', 'sáb']

// formatShortDate turns an API date ("2026-09-01") into "1 sept". It splits
// the string instead of parsing a Date: new Date("2026-09-01") is UTC
// midnight, which is the previous day west of UTC.
export function formatShortDate(iso: string): string {
  const dateSplit = iso.split('-')
  return `${Number(dateSplit[2])} ${MONTHS[Number(dateSplit[1]) - 1]}`
}

// formatSeasonRange formats a season's inclusive date range: "1 sept – 31 dic".
export function formatSeasonRange(from: string, to: string): string {
  return `${formatShortDate(from)} – ${formatShortDate(to)}`
}

// initials returns the upper-cased first letters of the first and last name
// ("AG" for Ana García), or fewer when a name is empty.
export function initials(first: string, last: string): string {
  const firstInitial = first.charAt(0)
  const lastInitial = last.charAt(0)
  return `${firstInitial ? firstInitial.toUpperCase() : ''}${lastInitial ? lastInitial.toUpperCase() : ''}`
}

// groupThousands puts a dot between groups of three digits ("2500" becomes
// "2.500"). By hand: Intl's es-ES format leaves 4-digit numbers ungrouped.
function groupThousands(n: number | string): string {
  return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, '.')
}

// formatDistance shows swims in whole metres ("2.500 m") and everything else
// in kilometres with one decimal ("54,2 km").
export function formatDistance(metres: number, sportType: string): string {
  if (sportType === 'Swim') {
    return groupThousands(Math.round(metres)) + ' m'
  }
  return groupThousands((metres / 1000).toFixed(1).split('.').join(',')) + ' km'
}

// formatDuration formats seconds as "45:00", or "1:02:05" from one hour on.
export function formatDuration(n: number): string {
  const h = Math.floor(n / 60 / 60)
  const m = String(Math.floor(n / 60) % 60).padStart(2, '0')
  const s = String(n % 60).padStart(2, '0')
  return `${h ? h + ':' : ''}${m}:${s}`
}

// formatDay turns "2026-10-26" into "lun 26 oct". new Date(y, m, d) is local
// midnight on that date, so the weekday is right in every time zone.
export function formatDay(date: string): string {
  const splt = date.split('-')
  const day = new Date(
    Number(splt[0]),
    Number(splt[1]) - 1,
    Number(splt[2]),
  ).getDay()
  return `${WEEKDAY[day]} ${formatShortDate(date)}`
}

// formatTime shows the athlete's local start time ("08:05") by slicing the
// API's wall-clock string: no Date, so it never shifts to the browser's zone.
export function formatTime(date: string): string {
  return date.slice(11, 16)
}

// Day groups activities that share a local start date, with the day's total.
export type Day<T> = { date: string; points: number; activities: T[] }

// groupByDay groups activities by date, keeping their order. The input is
// newest first (as the API sends it), so activities of one day are next to
// each other; a day split across two pages merges once both are loaded.
export function groupByDay<T extends { date: string; points: number }>(
  activities: T[],
): Day<T>[] {
  const days: Day<T>[] = []
  for (const a of activities) {
    const last = days.at(-1)
    if (last && last.date === a.date) {
      last.points += a.points
      last.activities.push(a)
    } else {
      days.push({ date: a.date, points: a.points, activities: [a] })
    }
  }
  return days
}

// SportKind picks the icon for an activity.
export type SportKind = 'run' | 'ride' | 'swim' | 'other'

// Spanish labels for the Strava sport types the app shows. Presentation only:
// which sports score is decided by the API (zeroReason).
const SPORTS: Record<string, { label: string; kind: SportKind }> = {
  Run: { label: 'Carrera', kind: 'run' },
  TrailRun: { label: 'Trail', kind: 'run' },
  VirtualRun: { label: 'Carrera virtual', kind: 'run' },
  Ride: { label: 'Ciclismo', kind: 'ride' },
  MountainBikeRide: { label: 'BTT', kind: 'ride' },
  GravelRide: { label: 'Gravel', kind: 'ride' },
  VirtualRide: { label: 'Ciclismo virtual', kind: 'ride' },
  EBikeRide: { label: 'Bici eléctrica', kind: 'ride' },
  EMountainBikeRide: { label: 'BTT eléctrica', kind: 'ride' },
  Swim: { label: 'Natación', kind: 'swim' },
  Walk: { label: 'Caminata', kind: 'other' },
}

// sport returns the label and icon kind of a Strava sport type; an unknown
// type is shown as Strava names it.
export function sport(sportType: string): { label: string; kind: SportKind } {
  return SPORTS[sportType] ?? { label: sportType, kind: 'other' }
}
