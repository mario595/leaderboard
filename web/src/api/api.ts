// ApiError is a non-2xx answer from our API, with its HTTP status.
export class ApiError extends Error {
  readonly status: number
  constructor(status: number) {
    super(`request failed with status ${status}`)
    this.name = 'ApiError'
    this.status = status
  }
}

export type RequestOptions = {
  signal?: AbortSignal
}

// getJSON fetches path and decodes its JSON body as T (trusted, not
// validated: it is our own API). A non-2xx answer throws an ApiError; a
// network failure or an abort rejects as fetch does.
export async function getJSON<T>(
  path: string,
  options: RequestOptions = {},
): Promise<T> {
  const res = await fetch(path, {
    headers: { Accept: 'application/json' },
    signal: options.signal,
  })
  if (!res.ok) {
    throw new ApiError(res.status)
  }
  return (await res.json()) as T
}

// Me is the body of GET /api/me.
export type Me = {
  id: number
  stravaId: number
  firstName: string
  lastName: string
}

export type Season = {
  id: number
  name: string
  startDate: string
  endDate: string
}

export type Standing = {
  rank: number
  userId: number
  firstName: string
  lastName: string
  points: number
}

export type Leaderboard = {
  season: Season
  standings: Standing[]
}

export type Activity = {
  id: number
  date: string
  startLocal: string
  sportType: string
  movingTime: number
  distance: number
  competition: boolean
  points: number
  zeroReason: string
}

export type ActivitiesPage = {
  season: Season
  activities: Activity[]
  nextCursor: string | null
}
