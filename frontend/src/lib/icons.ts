// icons.ts is the app's only source of OpenIcons (`oi oi-<name>`) classes;
// components use this map instead of spelling out a class string of their
// own.

import type { Translate } from '@/i18n'

type Tone = 'green' | 'blue' | 'red' | 'violet' | 'orange' | 'muted'

// Whole class names, so Tailwind finds them in the source.
export const TONE_CLASS: Record<Tone, string> = {
  green: 'text-tone-green',
  blue: 'text-tone-blue',
  red: 'text-tone-red',
  violet: 'text-tone-violet',
  orange: 'text-tone-orange',
  muted: 'text-tone-muted',
}

interface StatusIcon {
  icon: string
  label: string
  tone: Tone
}

interface StatusEntry {
  icon: string
  labelKey: string
  tone: Tone
}

const CONFLICTED: StatusEntry = {
  icon: 'exclamation-triangle',
  labelKey: 'conflicted',
  tone: 'orange',
}

const STATUS: Record<string, StatusEntry> = {
  A: { icon: 'file-plus', labelKey: 'added', tone: 'green' },
  M: { icon: 'file-edit', labelKey: 'modified', tone: 'blue' },
  D: { icon: 'trash', labelKey: 'deleted', tone: 'red' },
  R: { icon: 'file-export', labelKey: 'renamed', tone: 'violet' },
  C: { icon: 'copy', labelKey: 'copied', tone: 'violet' },
  T: { icon: 'sync', labelKey: 'typeChanged', tone: 'muted' },
  '?': { icon: 'question-circle', labelKey: 'untracked', tone: 'muted' },
  U: CONFLICTED,
}

// statusIcon maps a changed file's status to its icon, tooltip label and
// colour tone. git reports a conflicted entry as a two-letter code (e.g.
// "UU", "AU", "DD"); every two-letter code is treated as conflicted. A code
// this map does not recognise falls back to a plain file icon labelled with
// the code itself.
export function statusIcon(status: string, t: Translate): StatusIcon {
  const entry = STATUS[status] ?? (status.length === 2 ? CONFLICTED : undefined)
  return entry
    ? { icon: entry.icon, label: t(`common.fileStatus.${entry.labelKey}`), tone: entry.tone }
    : { icon: 'file', label: status, tone: 'muted' }
}

// ICON is every other icon the app draws, its app-level name to the
// OpenIcons name.
const ICON = {
  openEditor: 'pencil',
  stage: 'plus',
  unstage: 'minus',
  discard: 'undo',
  markResolved: 'check',
  reopenConflict: 'replay',
  close: 'times',
  clear: 'times',
  settings: 'cog',
  externalLink: 'external-link',
  removeWorktree: 'trash',
  repo: 'book',
  worktree: 'folder',
  folder: 'folder',
  fileList: 'list',
  fileTree: 'sitemap',
  foldAll: 'folder',
  unfoldAll: 'folder-open',
  branch: 'code-branch',
  dropdown: 'chevron-down',
  fetch: 'cloud-download',
  pull: 'download',
  push: 'upload',
  refresh: 'refresh',
  ahead: 'arrow-up',
  behind: 'arrow-down',
  prevChange: 'chevron-up',
  nextChange: 'chevron-down',
  prevFile: 'angle-double-up',
  nextFile: 'angle-double-down',
  unifiedDiff: 'align-justify',
  sideBySideDiff: 'th-large',
  current: 'check',
  locked: 'lock',
  dirty: 'circle-fill',
  swap: 'arrows-h',
  jobsIdle: 'clock',
  jobsRunning: 'spinner',
  abort: 'stop-circle',
  rowActions: 'ellipsis-h',
} as const

export type IconName = keyof typeof ICON

// oiClass builds the OpenIcons CSS class for a bare icon name, e.g.
// oiClass('cog') === 'oi oi-cog'.
export function oiClass(name: string): string {
  return `oi oi-${name}`
}

// iconClass is oiClass for one of the app's named icons.
export function iconClass(name: IconName): string {
  return oiClass(ICON[name])
}

// spinClass is iconClass plus OpenIcons' own spin animation, for a job in
// progress.
export function spinClass(name: IconName): string {
  return `${iconClass(name)} oi-spin`
}

// NOTICE_ICON is each notice severity's icon: the ones OpenVue's Toast
// draws itself.
const INFO_ICON = 'info-circle'

const NOTICE_ICON: Record<string, string> = {
  error: 'times-circle',
  warn: 'exclamation-triangle',
  info: INFO_ICON,
  success: 'check',
}

// noticeIconClass is a notice severity's icon class, info's when unset.
export function noticeIconClass(severity: string | undefined): string {
  return oiClass(NOTICE_ICON[severity ?? ''] ?? INFO_ICON)
}
