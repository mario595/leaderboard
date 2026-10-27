import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, getJSON } from './api'

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('getJSON', () => {
  it('returns the decoded body of a 2xx response', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { id: 7 }))
    vi.stubGlobal('fetch', fetchMock)

    const body = await getJSON<{ id: number }>('/api/me')

    expect(body).toEqual({ id: 7 })
    expect(fetchMock).toHaveBeenCalledWith('/api/me', expect.anything())
  })

  it('throws an ApiError with the status for a non-2xx response', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(jsonResponse(401, { error: 'unauthorized' })),
    )

    const err = await getJSON('/api/me').catch((e: unknown) => e)

    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).status).toBe(401)
  })

  it('lets a network failure through', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockRejectedValue(new TypeError('Failed to fetch')),
    )

    await expect(getJSON('/api/me')).rejects.toThrow('Failed to fetch')
  })
})

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}
