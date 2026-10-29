import { useEffect, useState } from 'react'
import { ApiError, getJSON } from './api'

// ApiState is what a page renders from: loading, the data, or an error with
// the HTTP status when the server answered (none for a network failure).
export type ApiState<T> =
  | { status: 'loading' }
  | { status: 'ready'; data: T }
  | { status: 'error'; httpStatus?: number }

// useApi loads path into an ApiState, again whenever path changes or retry is
// called (retry shows loading at once). The pending request is aborted when
// the component unmounts or the path changes, so a late answer never
// overwrites newer state; an aborted request is not an error.
export function useApi<T>(path: string): ApiState<T> & { retry: () => void } {
  const [state, setState] = useState<ApiState<T>>({ status: 'loading' })
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    const controller = new AbortController()

    getJSON<T>(path, { signal: controller.signal })
      .then((data) => setState({ status: 'ready', data }))
      .catch((err: unknown) => {
        if (err instanceof DOMException && err.name === 'AbortError') return
        setState({
          status: 'error',
          httpStatus: err instanceof ApiError ? err.status : undefined,
        })
      })
    return () => controller.abort()
  }, [path, attempt])

  function retry() {
    setState({ status: 'loading' })
    setAttempt((n) => n + 1)
  }
  return { ...state, retry }
}
