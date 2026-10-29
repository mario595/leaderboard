// Inline SVG icons from the design (docs/design). Decorative: aria-hidden,
// the text next to them carries the meaning. Stroke icons follow currentColor.

export function BrandIcon() {
  return (
    <svg className="icon icon--filled" viewBox="0 0 24 24" aria-hidden="true">
      <rect x="2.5" y="12" width="6" height="9" rx="1.5" />
      <rect x="9" y="4" width="6" height="17" rx="1.5" />
      <rect x="15.5" y="8.5" width="6" height="12.5" rx="1.5" />
    </svg>
  )
}

export function AlertIcon() {
  return (
    <svg className="icon" viewBox="0 0 24 24" aria-hidden="true">
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7.5v5.5M12 16.5v.01" />
    </svg>
  )
}

export function LogoutIcon() {
  return (
    <svg className="icon icon--small" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M15 4h3a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2h-3M10 16l-4-4 4-4M6 12h10" />
    </svg>
  )
}

export function StandingsIcon() {
  return (
    <svg className="icon" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M3 20.5h18M4.5 20.5v-7h5v7M9.5 20.5V5h5v15.5M14.5 20.5v-10h5v10" />
    </svg>
  )
}

export function ActivityIcon() {
  return (
    <svg className="icon" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M3 12h4l3-7 4 14 3-7h4" />
    </svg>
  )
}

export function InfoIcon() {
  return (
    <svg className="icon" viewBox="0 0 24 24" aria-hidden="true">
      <circle cx="12" cy="12" r="9" />
      <path d="M12 11v5.5M12 7.5v.01" />
    </svg>
  )
}

export function CalendarIcon() {
  return (
    <svg className="icon" viewBox="0 0 24 24" aria-hidden="true">
      <rect x="3.5" y="5" width="17" height="15" rx="2" />
      <path d="M3.5 10h17M8 3v4M16 3v4" />
    </svg>
  )
}

export function RunIcon() {
  return (
    <svg className="icon" viewBox="0 0 24 24" aria-hidden="true">
      <circle cx="14.5" cy="4.5" r="2" />
      <path d="M6 11.5l3.5-3h4.5l2.5 4 3 1M10.5 8.5L9 15l4 2.5V22M9 15l-2.5 6" />
    </svg>
  )
}

export function RideIcon() {
  return (
    <svg className="icon" viewBox="0 0 24 24" aria-hidden="true">
      <circle cx="5.5" cy="16.5" r="3.5" />
      <circle cx="18.5" cy="16.5" r="3.5" />
      <path d="M5.5 16.5L9 9h6.5l3 7.5M9 9l3.5 7.5h-7M13.5 5.5h3l-1 3.5" />
    </svg>
  )
}

export function SwimIcon() {
  return (
    <svg className="icon" viewBox="0 0 24 24" aria-hidden="true">
      <circle cx="16.5" cy="7.5" r="2" />
      <path d="M4 13l5.5-4 3 3.5M2.5 17.5c1.5 0 1.5-1 3-1s1.5 1 3 1 1.5-1 3-1 1.5 1 3 1 1.5-1 3-1 1.5 1 3 1" />
    </svg>
  )
}

export function TrophyIcon({ small = false }: { small?: boolean }) {
  return (
    <svg
      className={small ? 'icon icon--small' : 'icon'}
      viewBox="0 0 24 24"
      aria-hidden="true"
    >
      <path d="M8 4h8v5a4 4 0 0 1-8 0V4zM8 6H5a3 3 0 0 0 3 4M16 6h3a3 3 0 0 1-3 4M12 13v4M8.5 20.5h7M10 17h4v3.5h-4z" />
    </svg>
  )
}

export function StopwatchIcon() {
  return (
    <svg className="icon" viewBox="0 0 24 24" aria-hidden="true">
      <circle cx="12" cy="13" r="8" />
      <path d="M12 9v4l2.5 2M9.5 2.5h5" />
    </svg>
  )
}

export function RefreshIcon() {
  return (
    <svg className="icon" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M20 11a8 8 0 0 0-14.5-4.5M4 4v3h3M4 13a8 8 0 0 0 14.5 4.5M20 20v-3h-3" />
    </svg>
  )
}
