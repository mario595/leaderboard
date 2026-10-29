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
