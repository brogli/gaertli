import { formatTimeAgo } from '@vueuse/core'

// shortSha is a commit hash's first 7 characters, as git abbreviates it.
export function shortSha(sha: string): string {
  return sha.slice(0, 7)
}

// timeAgo is a git timestamp (Unix seconds) relative to now, e.g. "3 days ago".
export function timeAgo(unixSeconds: number): string {
  return formatTimeAgo(new Date(unixSeconds * 1000))
}
