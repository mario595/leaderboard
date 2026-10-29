import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useApi } from './useApi'

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('useApi', () => {
  it('is loading, then ready with the data', async () => {
    stubFetch(() => Response.json({ points: 6 }))

    const { result } = renderHook(() =>
      useApi<{ points: number }>('/api/thing'),
    )

    expect(result.current.status).toBe('loading')
    await waitFor(() => expect(result.current.status).toBe('ready'))
    expect(result.current).toMatchObject({
      status: 'ready',
      data: { points: 6 },
    })
  })

  it('reports the HTTP status of a failed response', async () => {
    stubFetch(() => Response.json({ error: 'no season' }, { status: 404 }))

    const { result } = renderHook(() => useApi('/api/thing'))

    await waitFor(() => expect(result.current.status).toBe('error'))
    expect(result.current).toMatchObject({ status: 'error', httpStatus: 404 })
  })

  it('reports a network failure without a status', async () => {
    stubFetch(() => {
      throw new TypeError('Failed to fetch')
    })

    const { result } = renderHook(() => useApi('/api/thing'))

    await waitFor(() => expect(result.current.status).toBe('error'))
    expect(result.current).not.toHaveProperty('httpStatus', expect.anything())
  })

  it('loads again on retry', async () => {
    let calls = 0
    const fetchMock = stubFetch(() => {
      calls++
      return calls === 1
        ? Response.json({ error: 'x' }, { status: 500 })
        : Response.json({ points: 6 })
    })
    const { result } = renderHook(() => useApi('/api/thing'))
    await waitFor(() => expect(result.current.status).toBe('error'))

    act(() => result.current.retry())

    // Back to loading straight away, so the page can show the skeleton.
    expect(result.current.status).toBe('loading')
    await waitFor(() => expect(result.current.status).toBe('ready'))
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('aborts the request when the component unmounts', async () => {
    const signals: AbortSignal[] = []
    stubFetch((signal) => {
      signals.push(signal)
      return new Promise<Response>(() => {}) // never answers
    })
    const { unmount } = renderHook(() => useApi('/api/thing'))
    await waitFor(() => expect(signals.length).toBeGreaterThan(0))

    unmount()

    // StrictMode is not used here, so there is exactly one request.
    expect(signals.at(-1)?.aborted).toBe(true)
  })

  it('aborts the old request and loads the new path when the path changes', async () => {
    const requests: { url: string; signal: AbortSignal }[] = []
    stubFetch((signal, url) => {
      requests.push({ url, signal })
      if (url === '/api/old') {
        // Answers only when aborted, like a real fetch: with an AbortError.
        return new Promise<Response>((_, reject) =>
          signal.addEventListener('abort', () =>
            reject(new DOMException('aborted', 'AbortError')),
          ),
        )
      }
      // The new path answers a little later, so an error set by the old,
      // aborted request would be visible in between.
      return new Promise<Response>((resolve) =>
        setTimeout(() => resolve(Response.json({ which: 'new' })), 50),
      )
    })
    const { result, rerender } = renderHook(({ path }) => useApi(path), {
      initialProps: { path: '/api/old' },
    })

    rerender({ path: '/api/new' })

    // The old request is aborted now; its AbortError must not show as an error
    // while the new request is still loading.
    await new Promise((resolve) => setTimeout(resolve, 20))
    expect(result.current.status).not.toBe('error')

    await waitFor(() => expect(result.current.status).toBe('ready'))
    expect(result.current).toMatchObject({ data: { which: 'new' } })
    expect(requests.find((r) => r.url === '/api/old')?.signal.aborted).toBe(
      true,
    )
  })
})

// stubFetch replaces fetch with fn, which gets the request's signal and URL.
// fn may return a Response, a Promise of one, or throw.
function stubFetch(
  fn: (signal: AbortSignal, url: string) => Response | Promise<Response>,
) {
  const fetchMock = vi.fn(
    async (input: RequestInfo | URL, init?: RequestInit) => {
      if (!init?.signal) throw new Error('fetch called without a signal')
      return fn(init.signal, String(input))
    },
  )
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}
