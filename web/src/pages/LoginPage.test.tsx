import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AppRoutes } from '../App'
import { AuthProvider } from '../auth/auth'

beforeEach(() => {
  // Nobody is logged in: /api/me answers 401.
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => Response.json({ error: 'x' }, { status: 401 })),
  )
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('LoginPage', () => {
  it('starts the Strava login with a full page load', async () => {
    renderLogin('/login')

    const button = await screen.findByRole('link', {
      name: /strava/i,
    })
    // A plain link to the Go server, not a client-side route: the OAuth
    // dance needs real navigation to Strava and back.
    expect(button).toHaveAttribute('href', '/auth/strava')
  })

  it('shows the scoring rules in short', async () => {
    renderLogin('/login')

    expect(await screen.findByText(/2 puntos/i)).toBeInTheDocument()
    expect(screen.getByText(/4 puntos/i)).toBeInTheDocument()
    expect(screen.getByText(/hasta 2 actividades al día/i)).toBeInTheDocument()
  })

  it('credits Strava', async () => {
    renderLogin('/login')

    // Strava's brand guidelines: their "Powered by Strava" logo.
    expect(
      await screen.findByRole('img', { name: /powered by strava/i }),
    ).toBeInTheDocument()
  })

  it('shows no error banner on a normal visit', async () => {
    renderLogin('/login')

    await screen.findByRole('link', { name: /strava/i })
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it.each([
    // Each reason the OAuth callback sends (#18 step 1) has its own message:
    // the user needs to know what to do differently.
    ['cancelled', /cancelado/i],
    ['scope', /permiso para ver tus actividades/i],
    ['failed', /inténtalo de nuevo/i],
    // An unknown reason (an old link, a typo) still explains something.
    ['something-else', /inténtalo de nuevo/i],
  ])('explains ?error=%s', async (code, message) => {
    renderLogin(`/login?error=${code}`)

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent(message)
    // The button stays usable: trying again is the way out.
    expect(screen.getByRole('link', { name: /strava/i })).toBeInTheDocument()
  })
})

function renderLogin(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </MemoryRouter>,
  )
}
