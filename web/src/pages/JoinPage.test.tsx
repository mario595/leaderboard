import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AppRoutes } from '../App'
import { AuthProvider } from '../auth/auth'

const ana = { id: 1, stravaId: 12345678, firstName: 'Ana', lastName: 'García' }

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('JoinPage', () => {
  it('renders for an anonymous visitor, with the footer', async () => {
    await renderJoin()

    expect(
      screen.getByRole('heading', { name: 'Antes de conectar tu Strava' }),
    ).toBeInTheDocument()
    expect(
      screen.getByRole('navigation', { name: 'Pie de página' }),
    ).toBeInTheDocument()
  })

  it('does not link to Strava until the athlete agrees', async () => {
    await renderJoin()

    // The button is there, but not as a link.
    expect(
      screen.getByRole('img', { name: 'Connect with Strava' }),
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('link', { name: 'Connect with Strava' }),
    ).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /strava/i })).toBeNull()
  })

  it('links to the Strava login once the athlete agrees', async () => {
    await renderJoin()

    await userEvent.click(screen.getByRole('checkbox'))

    expect(
      screen.getByRole('link', { name: 'Connect with Strava' }),
    ).toHaveAttribute('href', '/auth/strava')
  })

  it('disables the button again when the box is unticked', async () => {
    await renderJoin()

    await userEvent.click(screen.getByRole('checkbox'))
    await userEvent.click(screen.getByRole('checkbox'))

    expect(
      screen.queryByRole('link', { name: 'Connect with Strava' }),
    ).not.toBeInTheDocument()
  })

  it('asks for agreement to show the name and points to the group (5.13)', async () => {
    await renderJoin()

    expect(
      screen.getByRole('checkbox', {
        name: /mi nombre y mis puntos se muestren a los miembros del grupo/,
      }),
    ).not.toBeChecked()
    expect(
      screen.getByRole('link', { name: 'política de privacidad' }),
    ).toHaveAttribute('href', '/privacy')
  })

  // Strava API Policy 2.1 (i) to (v).
  it.each([
    ['the types of data', /el tiempo en movimiento, la distancia/],
    ['how it is collected', /Strava nos lo da al conectar tu cuenta/],
    [
      'how to withdraw consent',
      /retirando el acceso a la aplicación en Strava/,
    ],
    ['how to request deletion', /Pedir el borrado/],
    ['that deletion is confirmed', /Te confirmaremos el borrado/],
  ])('discloses %s', async (_, text) => {
    await renderJoin()

    expect(screen.getByText(text)).toBeInTheDocument()
  })

  it('gives the support address for deletion requests', async () => {
    await renderJoin()

    expect(
      screen.getByRole('link', { name: 'leaderboard.oxidant225@aleeas.com' }),
    ).toHaveAttribute('href', 'mailto:leaderboard.oxidant225@aleeas.com')
  })

  it('sends a logged-in member to the leaderboard', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) =>
        String(input) === '/api/me'
          ? Response.json(ana)
          : Response.json({ error: 'not found' }, { status: 404 }),
      ),
    )
    renderAt('/join')

    expect(
      await screen.findByRole('heading', { name: 'Clasificación' }),
    ).toBeInTheDocument()
  })
})

async function renderJoin() {
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => Response.json({ error: 'x' }, { status: 401 })),
  )
  renderAt('/join')
  // Anonymous, not sent to the login page.
  await screen.findByRole('heading', { name: 'Antes de conectar tu Strava' })
}

function renderAt(path: string) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </MemoryRouter>,
  )
}
