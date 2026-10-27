export class ApiError extends Error {
  readonly status: number
  constructor(status: number) {
    super(`request failed with status ${status}`)
    this.name = 'ApiError'
    this.status = status
  }
}

export async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(path, { headers: { Accept: 'application/json' } })
  if (!res.ok) {
    throw new ApiError(res.status)
  }
  return (await res.json()) as T
}

export type Me = {
  id: number
  stravaId: number
  firstName: string
  lastName: string
}
