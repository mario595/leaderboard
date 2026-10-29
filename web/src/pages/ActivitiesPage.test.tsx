import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AppRoutes } from '../App'
import { AuthProvider } from '../auth/auth'

const ana = { id: 1, stravaId: 12345678, firstName: 'Ana', lastName: 'García' }

const season = {
  id: 3,
  name: 'Otoño 2026',
  startDate: '2026-09-01',
  endDate: '2026-12-31',
}

const leaderboard = {
  season,
  standings: [
    { rank: 1, userId: 4, firstName: 'Luis', lastName: 'Pérez', points: 46 },
    { rank: 2, userId: 1, firstName: 'Ana', lastName: 'García', points: 40 },
  ],
}

type Activity = {
  id: number
  date: string
  startLocal: string
  sportType: string
  movingTime: number
  distance: number
  competition: boolean
  points: number
  zeroReason: string
}

function activity(fields: Partial<Activity> & { id: number; date: string }) {
  return {
    startLocal: `${fields.date}T08:00:00`,
    sportType: 'Run',
    movingTime: 2700,
    distance: 9000,
    competition: false,
    points: 2,
    zeroReason: '',
    ...fields,
  }
}

// First page: three days, one activity per reason for 0 points. 2026-10-26
// is a Monday, 2026-10-25 a Sunday, 2026-10-18 a Sunday, 2026-10-12 a Monday.
const page1 = {
  season,
  activities: [
    activity({
      id: 1611,
      date: '2026-10-26',
      startLocal: '2026-10-26T08:05:00',
      movingTime: 3725,
      distance: 12034.5,
      competition: true,
      points: 4,
    }),
    activity({
      id: 1602,
      date: '2026-10-25',
      startLocal: '2026-10-25T18:30:00',
      sportType: 'Swim',
      distance: 2500,
    }),
    activity({
      id: 1598,
      date: '2026-10-25',
      startLocal: '2026-10-25T07:00:00',
      sportType: 'GravelRide',
      movingTime: 7380,
      distance: 54210.3,
    }),
    activity({
      id: 1590,
      date: '2026-10-25',
      startLocal: '2026-10-25T06:10:00',
      points: 0,
      zeroReason: 'daily_cap',
    }),
    activity({
      id: 1566,
      date: '2026-10-18',
      sportType: 'EBikeRide',
      points: 0,
      zeroReason: 'sport',
    }),
  ],
  nextCursor: 'abc+/=',
}

// Second page: 2026-10-18 continues (its day must merge with the first
// page's), then an older day.
const page2 = {
  season,
  activities: [
    activity({
      id: 1562,
      date: '2026-10-18',
      startLocal: '2026-10-18T07:00:00',
      sportType: 'VirtualRide',
    }),
    activity({
      id: 1550,
      date: '2026-10-12',
      movingTime: 1500,
      points: 0,
      zeroReason: 'short',
    }),
  ],
  nextCursor: null,
}

const nextPageURL = '/api/me/activities?cursor=abc%2B%2F%3D'

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('ActivitiesPage', () => {
  it('shows the season, my name, my stats and the note', async () => {
    mockServer({ leaderboard, activities: [page1] })
    renderPage()

    expect(
      await screen.findByRole('heading', { name: 'Ana García', level: 1 }),
    ).toBeInTheDocument()
    expect(screen.getByText('Otoño 2026 · 1 sept – 31 dic')).toBeInTheDocument()
    expect(await screen.findByText('Tu posición')).toBeInTheDocument()
    expect(statValue('Tu posición')).toHaveTextContent('2º')
    expect(statValue('Tus puntos')).toHaveTextContent('40')
    expect(
      screen.getByText(/puntos pueden tardar hasta un par de horas/i),
    ).toBeInTheDocument()
  })

  it('groups activities by day, newest first, with each day’s points', async () => {
    mockServer({ leaderboard, activities: [page1] })
    renderPage()

    await screen.findByRole('heading', { name: 'Ana García', level: 1 })
    expect(dayHeadings()).toEqual([
      'lun 26 oct4 pts',
      'dom 25 oct4 pts',
      'dom 18 oct0 pts',
    ])
    expect(cardsOfDay('dom 25 oct').map((c) => cardSport(c))).toEqual([
      'Natación',
      'Gravel',
      'Carrera',
    ])
  })

  it('shows each activity’s sport, time, duration, distance and points', async () => {
    mockServer({ leaderboard, activities: [page1] })
    renderPage()

    await screen.findByRole('heading', { name: 'Ana García', level: 1 })
    const [race] = cardsOfDay('lun 26 oct')
    expect(race).toHaveTextContent('Carrera')
    // Exact text: the raw startLocal also contains "08:05".
    expect(within(race).getByText('08:05')).toBeInTheDocument()
    expect(race).toHaveTextContent('1:02:05')
    expect(race).toHaveTextContent('12,0 km')
    expect(race).toHaveTextContent(/4\s*pts$/)
    expect(within(race).getByText('Competición')).toBeInTheDocument()

    const [swim, gravel] = cardsOfDay('dom 25 oct')
    expect(swim).toHaveTextContent('2.500 m')
    expect(swim).toHaveTextContent('45:00')
    expect(within(swim).queryByText('Competición')).not.toBeInTheDocument()
    expect(gravel).toHaveTextContent('54,2 km')
    expect(gravel).toHaveTextContent('2:03:00')
  })

  it('de-emphasises 0-point activities and says why they scored nothing', async () => {
    mockServer({ leaderboard, activities: [page1, page2] })
    renderPage()

    await screen.findByRole('heading', { name: 'Ana García', level: 1 })
    await userEvent.click(screen.getByRole('button', { name: 'Cargar más' }))
    await screen.findByText('lun 12 oct')

    const zero = allCards().filter((c) =>
      c.classList.contains('activity-card--zero'),
    )
    expect(zero.map((c) => cardSport(c))).toEqual([
      'Carrera',
      'Bici eléctrica',
      'Carrera',
    ])
    expect(zero[0]).toHaveTextContent('Límite diario alcanzado')
    expect(zero[1]).toHaveTextContent('Deporte no puntuable')
    expect(zero[2]).toHaveTextContent('Menos de 30 min')

    // Scoring activities give no reason.
    const [race] = cardsOfDay('lun 26 oct')
    expect(race).not.toHaveClass('activity-card--zero')
    expect(race).not.toHaveTextContent(/límite|no puntuable|menos de/i)
  })

  it('appends the next page, merging a day split across pages', async () => {
    const fetchMock = mockServer({ leaderboard, activities: [page1, page2] })
    renderPage()

    await screen.findByRole('heading', { name: 'Ana García', level: 1 })
    await userEvent.click(screen.getByRole('button', { name: 'Cargar más' }))

    await screen.findByText('lun 12 oct')
    expect(fetchMock).toHaveBeenCalledWith(nextPageURL, expect.anything())
    // One 18 October, now with both activities and their points summed.
    expect(dayHeadings()).toEqual([
      'lun 26 oct4 pts',
      'dom 25 oct4 pts',
      'dom 18 oct2 pts',
      'lun 12 oct0 pts',
    ])
    expect(cardsOfDay('dom 18 oct').map((c) => cardSport(c))).toEqual([
      'Bici eléctrica',
      'Ciclismo virtual',
    ])
    // The last page: nothing more to load.
    expect(
      screen.queryByRole('button', { name: 'Cargar más' }),
    ).not.toBeInTheDocument()
  })

  it('disables the button while the next page loads', async () => {
    mockServer({ leaderboard, activities: [page1, 'never'] })
    renderPage()

    await screen.findByRole('heading', { name: 'Ana García', level: 1 })
    await userEvent.click(screen.getByRole('button', { name: 'Cargar más' }))

    expect(
      await screen.findByRole('button', { name: 'Cargando…' }),
    ).toBeDisabled()
  })

  it('keeps the list when loading more fails, and tries again', async () => {
    mockServer({ leaderboard, activities: [page1, 500, page2] })
    renderPage()

    await screen.findByRole('heading', { name: 'Ana García', level: 1 })
    await userEvent.click(screen.getByRole('button', { name: 'Cargar más' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      /no hemos podido cargar más actividades/i,
    )
    expect(dayHeadings()).toHaveLength(3)

    await userEvent.click(screen.getByRole('button', { name: 'Cargar más' }))
    await screen.findByText('lun 12 oct')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('still shows my activities when the stats fail to load', async () => {
    mockServer({ leaderboard: 500, activities: [page1] })
    renderPage()

    expect(await screen.findByText('lun 26 oct')).toBeInTheDocument()
    expect(screen.queryByText('Tu posición')).not.toBeInTheDocument()
  })

  it('says there are no activities yet, with a link to the rules', async () => {
    mockServer({
      leaderboard,
      activities: [{ season, activities: [], nextCursor: null }],
    })
    renderPage()

    expect(
      await screen.findByText(/aún no tienes actividades esta temporada/i),
    ).toBeInTheDocument()
    expect(
      screen.getByRole('link', { name: 'Ver cómo se puntúa' }),
    ).toHaveAttribute('href', '/rules')
    expect(
      screen.queryByRole('button', { name: 'Cargar más' }),
    ).not.toBeInTheDocument()
  })

  it('shows a loading state until the activities arrive', async () => {
    mockServer({ leaderboard, activities: ['never'] })
    renderPage()

    expect(await screen.findByRole('status')).toHaveTextContent(
      /cargando tus actividades/i,
    )
  })

  it('explains that there is no season running (404)', async () => {
    mockServer({ leaderboard: 404, activities: [404] })
    renderPage()

    expect(
      await screen.findByText(/no hay ninguna temporada en curso/i),
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: /reintentar/i }),
    ).not.toBeInTheDocument()
  })

  it('shows an error with a retry that loads again', async () => {
    mockServer({ leaderboard, activities: [500, page1] })
    renderPage()

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent(/algo ha fallado/i)

    await userEvent.click(
      within(alert).getByRole('button', { name: /reintentar/i }),
    )
    expect(await screen.findByText('lun 26 oct')).toBeInTheDocument()
  })
})

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/activities']}>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </MemoryRouter>,
  )
}

// dayHeadings returns the text of each day's heading (date and points).
function dayHeadings(): string[] {
  return screen
    .getAllByRole('heading', { level: 2 })
    .map((h) => h.textContent ?? '')
}

// cardsOfDay returns the activity cards under the day whose heading starts
// with label.
function cardsOfDay(label: string): HTMLElement[] {
  const heading = screen
    .getAllByRole('heading', { level: 2 })
    .find((h) => h.textContent?.startsWith(label))
  const day = heading?.closest('section')
  if (!day) throw new Error(`no day ${label}`)
  return within(day).getAllByRole('listitem')
}

function allCards(): HTMLElement[] {
  return screen
    .getAllByRole('heading', { level: 2 })
    .flatMap((h) => cardsOfDay(h.textContent ?? ''))
}

function cardSport(card: HTMLElement): string {
  return card.querySelector('.activity-card__sport')?.textContent ?? ''
}

// statValue returns the value next to a stat label (a <dt>/<dd> pair).
function statValue(label: string): HTMLElement {
  const value = screen.getByText(label).nextElementSibling
  if (!(value instanceof HTMLElement))
    throw new Error(`no value after ${label}`)
  return value
}

type Answer = object | number | 'never'

// mockServer stubs fetch: /api/me is Ana; /api/leaderboard answers with its
// body or status; /api/me/activities answers each call (first page, then
// each ?cursor= page) with the next item of activities.
function mockServer({
  leaderboard: board,
  activities,
}: {
  leaderboard: Answer
  activities: Answer[]
}) {
  let call = 0
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input)
    if (url === '/api/me') return Response.json(ana)
    if (url === '/api/leaderboard') return answer(board)
    if (url === '/api/me/activities' || url === nextPageURL) {
      return answer(activities[call++] ?? 404)
    }
    return Response.json({ error: 'not found' }, { status: 404 })
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

function answer(a: Answer): Promise<Response> | Response {
  if (a === 'never') return new Promise<Response>(() => {})
  if (typeof a === 'number') return Response.json({ error: 'x' }, { status: a })
  return Response.json(a)
}
