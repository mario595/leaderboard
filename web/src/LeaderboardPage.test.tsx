import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AppRoutes } from './App'
import { AuthProvider } from './auth'

const ana = { id: 1, stravaId: 12345678, firstName: 'Ana', lastName: 'García' }

// Already sorted and ranked by the API, ties sharing a rank (#16). The page
// must keep this order and these ranks: it never re-ranks.
const leaderboard = {
  season: {
    id: 3,
    name: 'Otoño 2026',
    startDate: '2026-09-01',
    endDate: '2026-12-31',
  },
  standings: [
    { rank: 1, userId: 4, firstName: 'Luis', lastName: 'Pérez', points: 46 },
    { rank: 2, userId: 7, firstName: 'Marta', lastName: 'Suárez', points: 40 },
    { rank: 2, userId: 1, firstName: 'Ana', lastName: 'García', points: 40 },
    { rank: 4, userId: 2, firstName: 'Jon', lastName: 'Ibarra', points: 22 },
    { rank: 5, userId: 9, firstName: 'Sara', lastName: 'Ruiz', points: 0 },
  ],
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('LeaderboardPage', () => {
  it('shows the season, my stats and the standings in API order', async () => {
    const fetchMock = mockServer({ leaderboard })
    renderPage()

    expect(
      await screen.findByText('Otoño 2026 · 1 sept – 31 dic'),
    ).toBeInTheDocument()
    expect(
      screen.getByRole('heading', { name: 'Clasificación' }),
    ).toBeInTheDocument()
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/leaderboard',
      expect.anything(),
    )

    // Stats: my rank (shared 2nd) and my points.
    expect(statValue('Tu posición')).toHaveTextContent('2º')
    expect(statValue('Tus puntos')).toHaveTextContent('40')

    const rows = standingsRows()
    expect(rows.map((r) => r.textContent)).toEqual([
      expect.stringMatching(/1.*LP.*Luis Pérez.*46/),
      expect.stringMatching(/2.*MS.*Marta Suárez.*40/),
      expect.stringMatching(/2.*AG.*Ana García.*40/),
      expect.stringMatching(/4.*JI.*Jon Ibarra.*22/),
      expect.stringMatching(/5.*SR.*Sara Ruiz.*0/),
    ])
  })

  it('marks my row, and only mine', async () => {
    mockServer({ leaderboard })
    renderPage()

    await screen.findByText('Otoño 2026 · 1 sept – 31 dic')
    const rows = standingsRows()
    const withBadge = rows.filter((r) => within(r).queryByText('Tú'))
    expect(withBadge).toHaveLength(1)
    expect(withBadge[0]).toHaveTextContent('Ana García')
    expect(withBadge[0]).toHaveClass('standings__row--me')
  })

  it('gives medals by rank, so tied members share one', async () => {
    mockServer({ leaderboard })
    const { container } = renderPage()

    await screen.findByText('Otoño 2026 · 1 sept – 31 dic')
    expect(container.querySelectorAll('.rank--gold')).toHaveLength(1)
    // Two members tied at 2: two silvers, and no bronze (nobody is 3rd).
    expect(container.querySelectorAll('.rank--silver')).toHaveLength(2)
    expect(container.querySelectorAll('.rank--bronze')).toHaveLength(0)
  })

  it('de-emphasises members with 0 points', async () => {
    mockServer({ leaderboard })
    renderPage()

    await screen.findByText('Otoño 2026 · 1 sept – 31 dic')
    const zero = standingsRows().filter((r) =>
      r.classList.contains('standings__row--zero'),
    )
    expect(zero).toHaveLength(1)
    expect(zero[0]).toHaveTextContent('Sara Ruiz')
  })

  it('links to the scoring rules below the standings', async () => {
    // Not the top-bar link ("Cómo se puntúa"): the question under the table.
    mockServer({ leaderboard })
    renderPage()

    expect(
      await screen.findByRole('link', { name: '¿Cómo se puntúa?' }),
    ).toBeInTheDocument()
  })

  it('shows a loading state until the standings arrive', async () => {
    mockServer({ leaderboard: 'never' })
    renderPage()

    expect(await screen.findByRole('status')).toHaveTextContent(
      /cargando la clasificación/i,
    )
    expect(
      screen.queryByRole('list', { name: /clasificación de la temporada/i }),
    ).not.toBeInTheDocument()
  })

  it('explains that there is no season running (404)', async () => {
    mockServer({ leaderboard: 404 })
    renderPage()

    expect(
      await screen.findByText(/no hay ninguna temporada en curso/i),
    ).toBeInTheDocument()
    // Not an error: nothing to retry.
    expect(
      screen.queryByRole('button', { name: /reintentar/i }),
    ).not.toBeInTheDocument()
  })

  it('shows an error with a retry that loads again', async () => {
    let calls = 0
    mockServer({
      leaderboard: () => (++calls === 1 ? 500 : leaderboard),
    })
    renderPage()

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent(/algo ha fallado/i)

    await userEvent.click(
      within(alert).getByRole('button', { name: /reintentar/i }),
    )

    expect(
      await screen.findByText('Otoño 2026 · 1 sept – 31 dic'),
    ).toBeInTheDocument()
    await waitFor(() => expect(calls).toBe(2))
  })
})

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/']}>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </MemoryRouter>,
  )
}

function standingsRows(): HTMLElement[] {
  const list = screen.getByRole('list', {
    name: /clasificación de la temporada/i,
  })
  return within(list).getAllByRole('listitem')
}

// statValue returns the value next to a stat label (a <dt>/<dd> pair).
function statValue(label: string): HTMLElement {
  const term = screen.getByText(label)
  const value = term.nextElementSibling
  if (!(value instanceof HTMLElement))
    throw new Error(`no value after ${label}`)
  return value
}

type LeaderboardAnswer = typeof leaderboard | number | 'never'

// mockServer stubs fetch: /api/me is Ana; /api/leaderboard answers with the
// body, a status, never, or what a function returns for each call.
function mockServer({
  leaderboard: answer,
}: {
  leaderboard: LeaderboardAnswer | (() => LeaderboardAnswer)
}) {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input)
    if (url === '/api/me') return Response.json(ana)
    if (url === '/api/leaderboard') {
      const a = typeof answer === 'function' ? answer() : answer
      if (a === 'never') return new Promise<Response>(() => {})
      if (typeof a === 'number')
        return Response.json({ error: 'x' }, { status: a })
      return Response.json(a)
    }
    return Response.json({ error: 'not found' }, { status: 404 })
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}
