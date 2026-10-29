import type { ReactNode } from 'react'

// EmptyState is the centred message used when a page has nothing to show:
// no season, no activities, or an error (isError adds role="alert" and the
// error colours). children holds an optional action, such as a retry button.
export function EmptyState({
  icon,
  title,
  text,
  isError = false,
  children,
}: {
  icon: ReactNode
  title: string
  text: string
  isError?: boolean
  children?: ReactNode
}) {
  return (
    <div
      className={isError ? 'empty-state empty-state--error' : 'empty-state'}
      role={isError ? 'alert' : undefined}
    >
      <span className="empty-state__icon">{icon}</span>
      <p className="empty-state__title">{title}</p>
      <p className="empty-state__text">{text}</p>
      {children}
    </div>
  )
}
