// PageHeader is the title block at the top of every page, with an optional
// line above the title (the season, for example).
export function PageHeader({
  title,
  eyebrow,
}: {
  title: string
  eyebrow?: string
}) {
  return (
    <div className="page-header">
      {eyebrow && <p className="page-header__eyebrow">{eyebrow}</p>}
      <h1 className="page-header__title">{title}</h1>
    </div>
  )
}
