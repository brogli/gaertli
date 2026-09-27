import { afterEach, describe, expect, it, vi } from 'vitest'
import { i18n } from '@/i18n'
import { exactDate, timeAgo } from '../format'

const NOW = new Date('2026-09-24T12:00:00Z')
const threeDaysAgo = NOW.getTime() / 1000 - 3 * 24 * 3600

describe('timeAgo', () => {
  afterEach(() => {
    i18n.global.locale.value = 'en'
    vi.useRealTimers()
  })

  it('FR-UX-11 a relative date is in the UI language', () => {
    vi.useFakeTimers({ now: NOW })

    expect(timeAgo(threeDaysAgo)).toBe('3 days ago')
    // eslint-disable-next-line @typescript-eslint/consistent-type-assertions -- Intl formatting accepts any locale tag; the ref's type only allows this app's own configured locale
    i18n.global.locale.value = 'de' as 'en'
    expect(timeAgo(threeDaysAgo)).toBe('vor 3 Tagen')
  })

  it('FR-UX-11 a date under a minute old is "just now", from the catalogue', () => {
    vi.useFakeTimers({ now: NOW })

    expect(timeAgo(NOW.getTime() / 1000 - 20)).toBe('just now')
  })
})

describe('exactDate', () => {
  it('FR-UX-10 formats the date and time in the system date format', () => {
    const date = new Date('2026-09-24T12:34:56Z')
    const expected = new Intl.DateTimeFormat(undefined, {
      dateStyle: 'medium',
      timeStyle: 'medium',
    }).format(date)

    const result = exactDate(date.getTime() / 1000)

    expect(result).toBe(expected)
    expect(result).toContain('2026')
    expect(result).toMatch(/:56\b/)
  })
})
