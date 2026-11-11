import { render, screen, waitFor, within } from '@testing-library/react'
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
      await screen.findByRole('link', { name: 'Connect with Strava' }),
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

  it('sends an unknown path to the leaderboard', async () => {
    mockServer({ me: ana })

    renderAt('/nope/really')

    expect(
      await screen.findByRole('heading', { name: 'Clasificación' }),
    ).toBeInTheDocument()
  })

  it('sends an anonymous visitor on an unknown path to the login page', async () => {
    mockServer({ me: 401 })

    renderAt('/nope')

    expect(
      await screen.findByRole('link', { name: 'Connect with Strava' }),
    ).toHaveAttribute('href', '/auth/strava')
  })

  it('protects every page, not only the home page', async () => {
    mockServer({ me: 401 })

    renderAt('/activities')

    expect(
      await screen.findByRole('link', { name: 'Connect with Strava' }),
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
      screen.queryByRole('link', { name: 'Connect with Strava' }),
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
      screen.queryByRole('link', { name: 'Connect with Strava' }),
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
      await screen.findByRole('link', { name: 'Connect with Strava' }),
    ).toBeInTheDocument()
    await waitFor(() =>
      expect(server).toHaveBeenCalledWith(
        '/auth/logout',
        expect.objectContaining({ method: 'POST' }),
      ),
    )
  })
})

describe('public pages and the footer', () => {
  const support = 'mailto:leaderboard.oxidant225@aleeas.com'

  it.each([
    ['/privacy', 'Política de privacidad'],
    ['/terms', 'Términos de uso'],
  ])('renders %s for an anonymous visitor', async (path, heading) => {
    mockServer({ me: 401 })

    renderAt(path)

    expect(
      await screen.findByRole('heading', { level: 1, name: heading }),
    ).toBeInTheDocument()
    // Not sent to the login page.
    expect(
      screen.queryByRole('link', { name: 'Connect with Strava' }),
    ).not.toBeInTheDocument()
  })

  it('renders a public page for a logged-in member too', async () => {
    mockServer({ me: ana })

    renderAt('/privacy')

    expect(
      await screen.findByRole('heading', {
        level: 1,
        name: 'Política de privacidad',
      }),
    ).toBeInTheDocument()
  })

  it('does not wait for the session check on a public page', () => {
    // /api/me never answers: a public page must not depend on it.
    vi.stubGlobal(
      'fetch',
      vi.fn(() => new Promise(() => {})),
    )

    renderAt('/terms')

    expect(
      screen.getByRole('heading', { level: 1, name: 'Términos de uso' }),
    ).toBeInTheDocument()
    expect(footer()).toBeInTheDocument()
  })

  it.each([
    ['the login page', '/login'],
    ['a public page', '/privacy'],
  ])(
    'shows the footer without a Strava account link on %s for an anonymous visitor',
    async (_, path) => {
      const server = mockServer({ me: 401 })

      renderAt(path)
      // Wait until the session check has answered, so a missing account link
      // means "anonymous", not "still loading".
      await waitFor(() =>
        expect(server).toHaveBeenCalledWith('/api/me', expect.anything()),
      )
      await screen.findByRole('heading', { level: 1 })

      const links = within(await findFooter())
      expect(links.getByRole('link', { name: 'Privacidad' })).toHaveAttribute(
        'href',
        '/privacy',
      )
      expect(links.getByRole('link', { name: 'Términos' })).toHaveAttribute(
        'href',
        '/terms',
      )
      expect(links.getByRole('link', { name: 'Soporte' })).toHaveAttribute(
        'href',
        support,
      )
      expect(
        links.queryByRole('link', { name: 'Tu cuenta de Strava' }),
      ).not.toBeInTheDocument()
    },
  )

  it.each([
    ['the leaderboard', '/', 'Clasificación'],
    ['a public page', '/terms', 'Términos de uso'],
  ])(
    'links a logged-in member to their Strava account on %s',
    async (_, path, heading) => {
      mockServer({ me: ana })

      renderAt(path)
      await screen.findByRole('heading', { level: 1, name: heading })

      const links = within(await findFooter())
      expect(links.getByRole('link', { name: 'Privacidad' })).toHaveAttribute(
        'href',
        '/privacy',
      )
      expect(links.getByRole('link', { name: 'Soporte' })).toHaveAttribute(
        'href',
        support,
      )
      expect(
        await links.findByRole('link', { name: 'Tu cuenta de Strava' }),
      ).toHaveAttribute('href', 'https://www.strava.com/athletes/12345678')
    },
  )
})

// The footer is found by its navigation landmark, not by <footer>: a footer
// inside <main> (as on the login page) has no contentinfo role.
function footer() {
  return screen.getByRole('navigation', { name: 'Pie de página' })
}

function findFooter() {
  return screen.findByRole('navigation', { name: 'Pie de página' })
}

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
