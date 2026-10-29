import { useEffect, useRef, useState } from 'react'
import {
  type ActivitiesPage,
  type Activity,
  ApiError,
  getJSON,
  type Season,
} from './api'

// ActivitiesState is what the activities page renders from. Once ready, it
// holds every activity loaded so far, the cursor of the next page (null after
// the last one), and the state of the "load more" request.
export type ActivitiesState =
  | { status: 'loading' }
  | { status: 'error'; httpStatus?: number }
  | {
      status: 'ready'
      season: Season
      activities: Activity[]
      nextCursor: string | null
      loadingMore: boolean
      loadMoreFailed: boolean
    }

// useActivities loads the first page of my activities on mount (aborted on
// unmount) and appends the next page on loadMore. loadMore is ignored while a
// page is loading and after the last one; when it fails, the loaded activities
// stay and loadMoreFailed is set until the next try. retry reloads from the
// first page.
export function useActivities(): ActivitiesState & {
  retry: () => void
  hasMore: boolean
  loadMore: () => void
} {
  const [state, setState] = useState<ActivitiesState>({ status: 'loading' })
  const [attempt, setAttempt] = useState(0)

  const inFlight = useRef(false)

  useEffect(() => {
    const controller = new AbortController()

    getJSON<ActivitiesPage>('/api/me/activities', {
      signal: controller.signal,
    })
      .then((data) =>
        setState({
          status: 'ready',
          season: data.season,
          activities: data.activities,
          nextCursor: data.nextCursor,
          loadingMore: false,
          loadMoreFailed: false,
        }),
      )
      .catch((err: unknown) => {
        if (err instanceof DOMException && err.name === 'AbortError') return
        setState({
          status: 'error',
          httpStatus: err instanceof ApiError ? err.status : undefined,
        })
      })
    return () => controller.abort()
  }, [attempt])

  function retry() {
    setState({ status: 'loading' })
    setAttempt((n) => n + 1)
  }

  function loadMore() {
    if (state.status !== 'ready' || !state.nextCursor || inFlight.current)
      return
    inFlight.current = true
    const cursor = state.nextCursor
    setState((prev) =>
      prev.status === 'ready'
        ? { ...prev, loadingMore: true, loadMoreFailed: false }
        : prev,
    )

    getJSON<ActivitiesPage>(
      '/api/me/activities?cursor=' + encodeURIComponent(cursor),
    )
      .then((page) =>
        setState((prev) =>
          prev.status === 'ready'
            ? {
                ...prev,
                activities: [...prev.activities, ...page.activities],
                nextCursor: page.nextCursor,
                loadingMore: false,
                loadMoreFailed: false,
              }
            : prev,
        ),
      )
      .catch(() =>
        setState((prev) =>
          prev.status === 'ready'
            ? { ...prev, loadingMore: false, loadMoreFailed: true }
            : prev,
        ),
      )
      .finally(() => {
        inFlight.current = false
      })
  }
  const hasMore = state.status === 'ready' && state.nextCursor !== null

  return { ...state, hasMore, loadMore, retry }
}
