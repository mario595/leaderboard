import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AppRoutes } from '../App'
import { AuthProvider } from '../auth/auth'
import { formatDateTime, formatLongDate } from '../api/format'

const ana = { id: 1, stravaId: 12345678, firstName: 'Ana', lastName: 'García' }
const deletion = {
  deletedAt: '2026-11-11T15:04:05Z',
  backupsGoneBy: '2026-11-25',
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('account deletion', () => {
  it('asks for confirmation before deleting anything', async () => {
    const server = mockServer({ deleteStatus: 200 })
    await renderActivities()

    await userEvent.click(
      screen.getByRole('button', { name: 'Desconectar y borrar mis datos' }),
    )

    expect(
      screen.getByRole('alertdialog', {
        name: '¿Seguro que quieres borrar tus datos?',
      }),
    ).toBeInTheDocument()
    expect(deleteCalls(server)).toBe(0)
  })

  it('cancels without deleting', async () => {
    const server = mockServer({ deleteStatus: 200 })
    await renderActivities()

    await userEvent.click(
      screen.getByRole('button', { name: 'Desconectar y borrar mis datos' }),
    )
    await userEvent.click(screen.getByRole('button', { name: 'Cancelar' }))

    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(deleteCalls(server)).toBe(0)
  })

  it('deletes with DELETE /api/me and shows the written confirmation', async () => {
    const server = mockServer({ deleteStatus: 200 })
    await renderActivities()

    await userEvent.click(
      screen.getByRole('button', { name: 'Desconectar y borrar mis datos' }),
    )
    await userEvent.click(
      screen.getByRole('button', { name: 'Sí, borrar mis datos' }),
    )

    expect(
      await screen.findByRole('heading', {
        level: 1,
        name: 'Tus datos se han borrado',
      }),
    ).toBeInTheDocument()
    expect(deleteCalls(server)).toBe(1)
    // The tests run in Los Angeles (UTC-8 in November): local time, not UTC.
    expect(
      screen.getByText(/El 11 de noviembre de 2026, 07:04 borramos tu cuenta/),
    ).toBeInTheDocument()
    expect(
      screen.getByText(/como muy tarde el 25 de noviembre de 2026/),
    ).toBeInTheDocument()
    // Logged out: the footer has no Strava account link any more.
    expect(
      screen.queryByRole('link', { name: 'Tu cuenta de Strava' }),
    ).not.toBeInTheDocument()
  })

  it('keeps the member on the page when the deletion fails', async () => {
    mockServer({ deleteStatus: 500 })
    await renderActivities()

    await userEvent.click(
      screen.getByRole('button', { name: 'Desconectar y borrar mis datos' }),
    )
    await userEvent.click(
      screen.getByRole('button', { name: 'Sí, borrar mis datos' }),
    )

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'No hemos podido borrar tus datos',
    )
    expect(
      screen.getByRole('button', { name: 'Sí, borrar mis datos' }),
    ).toBeEnabled()
    expect(
      screen.queryByRole('heading', { name: 'Tus datos se han borrado' }),
    ).not.toBeInTheDocument()
  })

  it('shows the confirmation without dates when opened directly', async () => {
    mockServer({ deleteStatus: 200, me: 401 })
    render(
      <MemoryRouter initialEntries={['/account-deleted']}>
        <AuthProvider>
          <AppRoutes />
        </AuthProvider>
      </MemoryRouter>,
    )

    expect(
      await screen.findByRole('heading', {
        level: 1,
        name: 'Tus datos se han borrado',
      }),
    ).toBeInTheDocument()
    expect(screen.getByText(/14 días después del borrado/)).toBeInTheDocument()
  })
})

describe('date formats for the confirmation', () => {
  it('writes an API date out in full', () => {
    expect(formatLongDate('2026-11-25')).toBe('25 de noviembre de 2026')
    expect(formatLongDate('2027-01-05')).toBe('5 de enero de 2027')
  })

  it('shows an instant in local time', () => {
    // 01:30 UTC on 1 December is still 30 November in Los Angeles.
    expect(formatDateTime('2026-12-01T01:30:00Z')).toBe(
      '30 de noviembre de 2026, 17:30',
    )
  })
})

async function renderActivities() {
  render(
    <MemoryRouter initialEntries={['/activities']}>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </MemoryRouter>,
  )
  await screen.findByRole('button', { name: 'Desconectar y borrar mis datos' })
  // The member is logged in (the footer shows their Strava link).
  await waitFor(() =>
    expect(
      screen.getByRole('link', { name: 'Tu cuenta de Strava' }),
    ).toBeInTheDocument(),
  )
}

// mockServer answers GET /api/me with Ana (or the given status), DELETE
// /api/me with the confirmation (or deleteStatus), and 404 for the season,
// so the activities page shows its "no season" state.
function mockServer({
  deleteStatus,
  me = 200,
}: {
  deleteStatus: number
  me?: number
}) {
  const fetchMock = vi.fn(
    async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (url === '/api/me' && init?.method === 'DELETE') {
        return deleteStatus === 200
          ? Response.json(deletion)
          : Response.json({ error: 'x' }, { status: deleteStatus })
      }
      if (url === '/api/me') {
        return me === 200
          ? Response.json(ana)
          : Response.json({ error: 'x' }, { status: me })
      }
      return Response.json({ error: 'not found' }, { status: 404 })
    },
  )
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

function deleteCalls(server: ReturnType<typeof mockServer>): number {
  return server.mock.calls.filter(
    ([url, init]) => String(url) === '/api/me' && init?.method === 'DELETE',
  ).length
}
