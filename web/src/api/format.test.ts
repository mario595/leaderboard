import { describe, expect, it } from 'vitest'
import {
  formatDay,
  formatDistance,
  formatDuration,
  formatSeasonRange,
  formatShortDate,
  formatTime,
  groupByDay,
  initials,
  sport,
} from './format'

describe('formatShortDate', () => {
  it.each([
    ['2026-09-01', '1 sept'],
    ['2026-12-31', '31 dic'],
    ['2027-01-05', '5 ene'],
    ['2026-05-15', '15 may'],
  ])('%s is %s', (iso, want) => {
    expect(formatShortDate(iso)).toBe(want)
  })

  it('keeps the calendar date in every time zone', () => {
    // new Date('2026-09-01') would be UTC midnight, the 31st of August west
    // of UTC. The date must not depend on where the browser is.
    expect(formatShortDate('2026-09-01')).toBe('1 sept')
  })
})

describe('formatSeasonRange', () => {
  it('joins both dates with a spaced en dash', () => {
    expect(formatSeasonRange('2026-09-01', '2026-12-31')).toBe(
      '1 sept – 31 dic',
    )
  })
})

describe('initials', () => {
  it.each([
    ['Ana', 'García', 'AG'],
    ['ana', 'garcía', 'AG'],
    ['María José', 'Hernández-Villalobos', 'MH'],
    ['Íñigo', 'Ñúñez', 'ÍÑ'],
    ['Ana', '', 'A'],
    ['', '', ''],
  ])('%s %s is %s', (first, last, want) => {
    expect(initials(first, last)).toBe(want)
  })
})

describe('formatDuration', () => {
  it.each([
    [0, '00:00'],
    [59, '00:59'],
    [2700, '45:00'],
    [3599, '59:59'],
    // Hours only when there are some, minutes and seconds always two digits.
    [3600, '1:00:00'],
    [3725, '1:02:05'],
    [7380, '2:03:00'],
    [36000, '10:00:00'],
  ])('%i seconds is %s', (seconds, want) => {
    expect(formatDuration(seconds)).toBe(want)
  })
})

describe('formatDistance', () => {
  it.each([
    // Kilometres with one decimal and a comma, thousands with a dot.
    ['Run', 12034.5, '12,0 km'],
    ['GravelRide', 54210.3, '54,2 km'],
    ['Ride', 999, '1,0 km'],
    ['Ride', 1234567, '1.234,6 km'],
    ['Run', 0, '0,0 km'],
    // Swims in whole metres: 2.500 m, not 2,5 km.
    ['Swim', 2500, '2.500 m'],
    ['Swim', 750.4, '750 m'],
    ['Swim', 12345, '12.345 m'],
  ])('%s %f m is %s', (sportType, metres, want) => {
    expect(formatDistance(metres, sportType)).toBe(want)
  })
})

describe('formatDay', () => {
  it.each([
    ['2026-10-26', 'lun 26 oct'],
    ['2026-10-25', 'dom 25 oct'],
    ['2026-11-01', 'dom 1 nov'],
    ['2027-01-02', 'sáb 2 ene'],
  ])('%s is %s', (date, want) => {
    // The tests run west of UTC: a Date parsed from "YYYY-MM-DD" would be
    // the previous day, with the previous weekday.
    expect(formatDay(date)).toBe(want)
  })
})

describe('formatTime', () => {
  it.each([
    ['2026-10-26T08:05:00', '08:05'],
    ['2026-10-26T23:59:59', '23:59'],
    ['2026-10-26T00:00:00', '00:00'],
  ])('%s is %s', (startLocal, want) => {
    // The athlete's wall clock: never shifted to the browser's zone.
    expect(formatTime(startLocal)).toBe(want)
  })
})

describe('sport', () => {
  it.each([
    ['Run', 'Carrera', 'run'],
    ['TrailRun', 'Trail', 'run'],
    ['VirtualRun', 'Carrera virtual', 'run'],
    ['Ride', 'Ciclismo', 'ride'],
    ['MountainBikeRide', 'BTT', 'ride'],
    ['GravelRide', 'Gravel', 'ride'],
    ['VirtualRide', 'Ciclismo virtual', 'ride'],
    ['EBikeRide', 'Bici eléctrica', 'ride'],
    ['EMountainBikeRide', 'BTT eléctrica', 'ride'],
    ['Swim', 'Natación', 'swim'],
    ['Walk', 'Caminata', 'other'],
    // A type the app does not know yet: shown as Strava sends it.
    ['Kitesurf', 'Kitesurf', 'other'],
  ])('%s is %s (%s)', (sportType, label, kind) => {
    expect(sport(sportType)).toEqual({ label, kind })
  })
})

describe('groupByDay', () => {
  it('groups consecutive activities by date, keeping the order', () => {
    const activities = [
      { id: 5, date: '2026-10-26', points: 4 },
      { id: 4, date: '2026-10-25', points: 2 },
      { id: 3, date: '2026-10-25', points: 2 },
      { id: 2, date: '2026-10-25', points: 0 },
      { id: 1, date: '2026-10-23', points: 0 },
    ]

    const days = groupByDay(activities)

    expect(days.map((d) => d.date)).toEqual([
      '2026-10-26',
      '2026-10-25',
      '2026-10-23',
    ])
    // The day's points are the sum of its activities (a 0-point day is 0).
    expect(days.map((d) => d.points)).toEqual([4, 4, 0])
    expect(days.map((d) => d.activities.map((a) => a.id))).toEqual([
      [5],
      [4, 3, 2],
      [1],
    ])
  })

  it('returns no days for no activities', () => {
    expect(groupByDay([])).toEqual([])
  })
})
