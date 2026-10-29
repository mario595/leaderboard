import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useActivities } from './useActivities'

const season = {
  id: 3,
  name: 'Otoño 2026',
  startDate: '2026-09-01',
  endDate: '2026-12-31',
}

function activity(id: number, date: string) {
  return {
    id,
    date,
    startLocal: `${date}T08:00:00`,
    sportType: 'Run',
    movingTime: 2700,
    distance: 9000,
    competition: false,
    points: 2,
    zeroReason: '',
  }
}

// Two pages: the first ends with a cursor (the next one exists), the second
// with null (the last one). The cursor contains characters that must be
// escaped in a URL.
const page1 = {
  season,
  activities: [activity(5, '2026-10-26'), activity(4, '2026-10-25')],
  nextCursor: 'abc+/=',
}
const page2 = {
  season,
  activities: [activity(3, '2026-10-25'), activity(2, '2026-10-20')],
  nextCursor: null,
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('useActivities', () => {
  it('loads the first page', async () => {
    const fetchMock = stubPages()

    const { result } = renderHook(() => useActivities())

    expect(result.current.status).toBe('loading')
    await waitFor(() => expect(result.current.status).toBe('ready'))
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/me/activities',
      expect.anything(),
    )
    expect(result.current).toMatchObject({
      status: 'ready',
      season,
      hasMore: true,
      loadingMore: false,
      loadMoreFailed: false,
    })
    expect(ids(result.current)).toEqual([5, 4])
  })

  it('appends the next page, asking with the escaped cursor', async () => {
    const fetchMock = stubPages()
    const { result } = renderHook(() => useActivities())
    await waitFor(() => expect(result.current.status).toBe('ready'))

    act(() => result.current.loadMore())

    expect(result.current).toMatchObject({ loadingMore: true })
    await waitFor(() =>
      expect(result.current).toMatchObject({ loadingMore: false }),
    )
    expect(fetchMock).toHaveBeenLastCalledWith(
      '/api/me/activities?cursor=abc%2B%2F%3D',
      expect.anything(),
    )
    // Appended in order, and the last page says there is nothing more.
    expect(ids(result.current)).toEqual([5, 4, 3, 2])
    expect(result.current).toMatchObject({ hasMore: false })
  })

  it('ignores a second loadMore while one is running', async () => {
    const fetchMock = stubPages({ slowSecondPage: true })
    const { result } = renderHook(() => useActivities())
    await waitFor(() => expect(result.current.status).toBe('ready'))

    act(() => {
      result.current.loadMore()
      result.current.loadMore()
    })
    await waitFor(() =>
      expect(result.current).toMatchObject({ loadingMore: false }),
    )

    // One first page, one next page: no duplicate rows.
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(ids(result.current)).toEqual([5, 4, 3, 2])
  })

  it('does nothing on loadMore after the last page', async () => {
    const fetchMock = stubPages({ onePage: true })
    const { result } = renderHook(() => useActivities())
    await waitFor(() => expect(result.current.status).toBe('ready'))
    expect(result.current).toMatchObject({ hasMore: false })

    act(() => result.current.loadMore())

    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('keeps the list when loading more fails, and tries again', async () => {
    let failNext = true
    stubPages({
      secondPage: () => {
        if (failNext) {
          failNext = false
          return Response.json({ error: 'x' }, { status: 500 })
        }
        return Response.json(page2)
      },
    })
    const { result } = renderHook(() => useActivities())
    await waitFor(() => expect(result.current.status).toBe('ready'))

    act(() => result.current.loadMore())
    await waitFor(() =>
      expect(result.current).toMatchObject({ loadMoreFailed: true }),
    )
    // What was shown stays shown.
    expect(result.current.status).toBe('ready')
    expect(ids(result.current)).toEqual([5, 4])

    act(() => result.current.loadMore())
    // Trying again clears the failure at once, not only when it succeeds.
    expect(result.current).toMatchObject({
      loadingMore: true,
      loadMoreFailed: false,
    })
    await waitFor(() =>
      expect(result.current).toMatchObject({
        loadingMore: false,
        loadMoreFailed: false,
      }),
    )
    expect(ids(result.current)).toEqual([5, 4, 3, 2])
  })

  it('reports a failed first page with its status, and retries', async () => {
    let calls = 0
    stubPages({
      firstPage: () =>
        ++calls === 1
          ? Response.json({ error: 'no season' }, { status: 404 })
          : Response.json(page1),
    })
    const { result } = renderHook(() => useActivities())

    await waitFor(() => expect(result.current.status).toBe('error'))
    expect(result.current).toMatchObject({ httpStatus: 404 })

    act(() => result.current.retry())

    expect(result.current.status).toBe('loading')
    await waitFor(() => expect(result.current.status).toBe('ready'))
    expect(ids(result.current)).toEqual([5, 4])
  })
})

function ids(state: ReturnType<typeof useActivities>): number[] {
  return state.status === 'ready' ? state.activities.map((a) => a.id) : []
}

// stubPages fakes GET /api/me/activities: the first page without a cursor,
// the second with one. Options replace either answer or slow the second.
function stubPages(
  opts: {
    onePage?: boolean
    slowSecondPage?: boolean
    firstPage?: () => Response
    secondPage?: () => Response
  } = {},
) {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input)
    if (url === '/api/me/activities') {
      if (opts.firstPage) return opts.firstPage()
      return Response.json(
        opts.onePage ? { ...page1, nextCursor: null } : page1,
      )
    }
    if (url.startsWith('/api/me/activities?cursor=')) {
      if (opts.secondPage) return opts.secondPage()
      if (opts.slowSecondPage) {
        await new Promise((resolve) => setTimeout(resolve, 30))
      }
      return Response.json(page2)
    }
    return Response.json({ error: 'not found' }, { status: 404 })
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}
