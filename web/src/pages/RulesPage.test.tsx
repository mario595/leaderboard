import { render, screen, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AppRoutes } from '../App'
import { AuthProvider } from '../auth/auth'

const ana = { id: 1, stravaId: 12345678, firstName: 'Ana', lastName: 'García' }

const leaderboard = {
  season: {
    id: 3,
    name: 'Otoño 2026',
    startDate: '2026-09-01',
    endDate: '2026-12-31',
  },
  standings: [],
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('RulesPage', () => {
  it('explains every scoring rule', async () => {
    mockServer(leaderboard)
    renderPage()

    expect(
      await screen.findByRole('heading', { name: 'Cómo se puntúa', level: 1 }),
    ).toBeInTheDocument()
    expect(
      screen.getAllByRole('heading', { level: 2 }).map((h) => h.textContent),
    ).toEqual([
      'Deportes que cuentan',
      'Al menos 30 minutos',
      '2 puntos, o 4 si compites',
      '¿Qué es una competición?',
      'Máximo 2 actividades al día',
      'Solo dentro de la temporada',
      '¿Cambiaste algo en Strava?',
    ])
  })

  it('lists the sports that count, and only those', async () => {
    mockServer(leaderboard)
    renderPage()

    const list = await screen.findByRole('list', {
      name: 'Deportes que cuentan',
    })
    expect(
      within(list)
        .getAllByRole('listitem')
        .map((li) => li.textContent),
    ).toEqual([
      'Carrera',
      'Trail',
      'Carrera virtual',
      'Ciclismo',
      'BTT',
      'Gravel',
      'Ciclismo virtual',
      'Natación',
    ])
  })

  it('shows the current season', async () => {
    mockServer(leaderboard)
    renderPage()

    expect(
      await screen.findByText('Otoño 2026, del 1 sept al 31 dic'),
    ).toBeInTheDocument()
    expect(screen.getByText('Otoño 2026 · 1 sept – 31 dic')).toBeInTheDocument()
  })

  it('still explains the rules when there is no season', async () => {
    const fetchMock = mockServer(404)
    renderPage()

    expect(
      await screen.findByRole('heading', {
        name: 'Solo dentro de la temporada',
      }),
    ).toBeInTheDocument()
    await vi.waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/leaderboard',
        expect.anything(),
      ),
    )
    expect(screen.queryByText(/Ahora:/)).not.toBeInTheDocument()
  })
})

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/rules']}>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </MemoryRouter>,
  )
}

// mockServer stubs fetch: /api/me is Ana; /api/leaderboard answers with the
// body or the status.
function mockServer(board: typeof leaderboard | number) {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input)
    if (url === '/api/me') return Response.json(ana)
    if (url === '/api/leaderboard') {
      return typeof board === 'number'
        ? Response.json({ error: 'x' }, { status: board })
        : Response.json(board)
    }
    return Response.json({ error: 'not found' }, { status: 404 })
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}
