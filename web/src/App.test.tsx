import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AppRoutes } from './App'
import { AuthProvider } from './auth/auth'

const ana = { id: 1, stravaId: 12345678, firstName: 'Ana', lastName: 'García' }

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('routing by auth state', () => {
  it('sends an anonymous visitor to the login page', async () => {
    mockServer({ me: 401 })

    renderAt('/')

    expect(
      await screen.findByRole('link', { name: /strava/i }),
    ).toHaveAttribute('href', '/auth/strava')
    expect(
      screen.queryByRole('heading', { name: 'Clasificación' }),
    ).not.toBeInTheDocument()
  })

  it('shows the leaderboard and the user name when logged in', async () => {
    mockServer({ me: ana })

    renderAt('/')

    expect(
      await screen.findByRole('heading', { name: 'Clasificación' }),
    ).toBeInTheDocument()
    expect(screen.getByText('Ana')).toBeInTheDocument()
  })

  it('sends a logged-in user away from the login page', async () => {
    mockServer({ me: ana })

    renderAt('/login')

    expect(
      await screen.findByRole('heading', { name: 'Clasificación' }),
    ).toBeInTheDocument()
  })

  it.each([
    ['/activities', 'Mis actividades'],
    ['/rules', 'Cómo se puntúa'],
  ])('renders %s for a logged-in user', async (path, heading) => {
    mockServer({ me: ana })

    renderAt(path)

    expect(
      await screen.findByRole('heading', { name: heading }),
    ).toBeInTheDocument()
  })

  it('protects every page, not only the home page', async () => {
    mockServer({ me: 401 })

    renderAt('/activities')

    expect(
      await screen.findByRole('link', { name: /strava/i }),
    ).toBeInTheDocument()
  })

  it('shows neither page while /api/me is still loading', () => {
    // A request that never answers: no flash of the login page for a user
    // who is in fact logged in.
    vi.stubGlobal(
      'fetch',
      vi.fn(() => new Promise(() => {})),
    )

    renderAt('/')

    expect(
      screen.queryByRole('link', { name: /strava/i }),
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('heading', { name: 'Clasificación' }),
    ).not.toBeInTheDocument()
  })

  it('shows an error, not the login page, when /api/me fails', async () => {
    // A 500 is not "logged out": showing the login page would be a lie.
    mockServer({ me: 500 })

    renderAt('/')

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(
      screen.queryByRole('link', { name: /strava/i }),
    ).not.toBeInTheDocument()
  })

  it('logs out with a POST and returns to the login page', async () => {
    const server = mockServer({ me: ana })
    renderAt('/')
    await screen.findByRole('heading', { name: 'Clasificación' })

    await userEvent.click(
      screen.getByRole('button', { name: /cerrar sesión/i }),
    )

    expect(
      await screen.findByRole('link', { name: /strava/i }),
    ).toBeInTheDocument()
    await waitFor(() =>
      expect(server).toHaveBeenCalledWith(
        '/auth/logout',
        expect.objectContaining({ method: 'POST' }),
      ),
    )
  })
})

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </MemoryRouter>,
  )
}

// mockServer stubs fetch: GET /api/me answers with the user, or with the
// given status; POST /auth/logout answers 204.
function mockServer({ me }: { me: typeof ana | number }) {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input)
    if (url === '/auth/logout') return new Response(null, { status: 204 })
    if (url === '/api/me') {
      return typeof me === 'number'
        ? Response.json({ error: 'x' }, { status: me })
        : Response.json(me)
    }
    return Response.json({ error: 'not found' }, { status: 404 })
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}
