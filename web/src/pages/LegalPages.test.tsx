import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AppRoutes } from '../App'
import { AuthProvider } from '../auth/auth'

afterEach(() => {
  vi.unstubAllGlobals()
})

// The pages are static, but the review checks them clause by clause: these
// tests pin the parts the Strava API Policy and the GDPR require, so an edit
// cannot drop one silently. Values are literals, not the constants.
describe('PrivacyPage', () => {
  it('names the controller and the contact', async () => {
    await renderPage('/privacy', 'Política de privacidad')

    expect(screen.getByText(/Mario Martinez Campos/)).toBeInTheDocument()
    expect(
      screen.getAllByRole('link', {
        name: 'leaderboard.oxidant225@aleeas.com',
      })[0],
    ).toHaveAttribute('href', 'mailto:leaderboard.oxidant225@aleeas.com')
  })

  it.each([
    ['what is collected', /tiempo en movimiento/],
    ['the description is never shown', /nunca se muestra a nadie/],
    ['the legal basis', /artículo 6\.1\.a/],
    ['who sees what', /Tus actividades solo las ves tú/],
    ['the host and region', /región de París/],
    ['the backup retention', /14 días/],
    ['the complaint authority', /Agencia Española de Protección de Datos/],
    [
      'the Strava usage data statement (6.5)',
      /fin empresarial, interno o externo/,
    ],
    ['new consent when the data changes (7.2)', /de nuevo tu consentimiento/],
  ])('covers %s', async (_, text) => {
    await renderPage('/privacy', 'Política de privacidad')

    expect(screen.getAllByText(text).length).toBeGreaterThan(0)
  })

  it('defers to the Strava privacy policy (7.3)', async () => {
    await renderPage('/privacy', 'Política de privacidad')

    expect(
      screen.getByRole('link', { name: 'Política de privacidad de Strava' }),
    ).toHaveAttribute('href', 'https://www.strava.com/legal/privacy')
    expect(
      screen.getByText(/prevalece en caso de conflicto/),
    ).toBeInTheDocument()
  })

  it('says how to withdraw consent on Strava', async () => {
    await renderPage('/privacy', 'Política de privacidad')

    expect(
      screen.getByRole('link', { name: /Mis aplicaciones/ }),
    ).toHaveAttribute('href', 'https://www.strava.com/settings/apps')
  })
})

describe('TermsPage', () => {
  it.each([
    ['private and non-commercial', /sin ánimo de lucro/],
    ['no conflict with Strava (9.1)', /prevalecen\s+las de Strava/],
    [
      'the third-party warranty disclaimer (9.2)',
      /comerciabilidad, de idoneidad para un fin determinado y de no infracción/,
    ],
    [
      'the third-party liability exclusion (9.2)',
      /indirecto, consecuente, especial o punitivo/,
    ],
  ])('covers %s', async (_, text) => {
    await renderPage('/terms', 'Términos de uso')

    expect(screen.getByText(text)).toBeInTheDocument()
  })

  it('links to the Strava terms and the privacy policy', async () => {
    await renderPage('/terms', 'Términos de uso')

    expect(
      screen.getByRole('link', { name: 'Condiciones de Strava' }),
    ).toHaveAttribute('href', 'https://www.strava.com/legal/terms')
    expect(
      screen.getByRole('link', { name: 'política de privacidad' }),
    ).toHaveAttribute('href', '/privacy')
  })
})

async function renderPage(path: string, heading: string) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => Response.json({ error: 'x' }, { status: 401 })),
  )
  render(
    <MemoryRouter initialEntries={[path]}>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </MemoryRouter>,
  )
  await screen.findByRole('heading', { level: 1, name: heading })
}
