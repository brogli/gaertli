import { formatTimeAgoIntl } from '@vueuse/core'
import { i18n } from '@/i18n'

// shortSha is a commit hash's first 7 characters, as git abbreviates it.
export function shortSha(sha: string): string {
  return sha.slice(0, 7)
}

const MINUTE_MS = 60_000

// timeAgo is a git timestamp (Unix seconds) relative to now in the UI's
// language, e.g. "3 days ago"; "just now" under a minute.
export function timeAgo(unixSeconds: number): string {
  const date = new Date(unixSeconds * 1000)
  const { t, locale } = i18n.global
  return Date.now() - date.getTime() < MINUTE_MS
    ? t('common.justNow')
    : formatTimeAgoIntl(date, { locale: locale.value })
}

// exactDate is a git timestamp (Unix seconds) as the exact date and time, in
// the user's system date format.
export function exactDate(unixSeconds: number): string {
  const date = new Date(unixSeconds * 1000)
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' }).format(
    date,
  )
}
