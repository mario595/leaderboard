import { describe, expect, it } from 'vitest'
import { formatSeasonRange, formatShortDate, initials } from './format'

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
