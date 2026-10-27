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
