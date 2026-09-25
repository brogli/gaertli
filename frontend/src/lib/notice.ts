import type { ToastMessageOptions } from 'openvue/toast'
import { useToast } from 'openvue/usetoast'
import { i18n } from '../i18n'
import type { CoreError } from '../composables/useCoreCall'
import { errorHeadline } from './errorText'

// NOTICE_GROUP is the group of the app's one notice area (NoticeArea.vue);
// every notice is added to it.
export const NOTICE_GROUP = 'app'

// NOTICE_LIFE (ms) is how long a notice stays before it closes itself, for
// those autoLife lets close.
const NOTICE_LIFE = 5000

// NoticeAction is a notice's recovery button. Its click leaves the notice
// open: what it starts replaces the notice (a fresh job, through
// useJobNotices).
export interface NoticeAction {
  label: string
  run: () => void
}

// AppNotice is a notice with the extras NoticeArea renders below its
// summary: copyText adds a button copying it, actions add recovery buttons,
// error adds its details toggle (git's command and stderr).
export interface AppNotice extends ToastMessageOptions {
  data?: { copyText?: string; actions?: NoticeAction[]; error?: CoreError }
}

// errorNotice is error's notice: its headline as the summary, its details
// and actions below. A retry error leads with "Try again", ahead of any
// action-specific ones.
export function errorNotice(error: CoreError, actions: NoticeAction[] = []): AppNotice {
  const retry = error.retry ? [{ label: i18n.global.t('common.tryAgain'), run: error.retry }] : []
  const all = [...retry, ...actions]
  return {
    severity: 'error',
    summary: errorHeadline(error),
    data: { error, ...(all.length > 0 && { actions: all }) },
  }
}

// shown maps a kind of notice to the one showing. Module state: it
// outlives the component that showed the notice, a dialog closing at once.
const shown = new Map<string, string | number>()

// useNotices shows at most one notice per kind: a newer one replaces only
// its own kind's, never another action's.
export function useNotices() {
  const toast = useToast()

  function clear(kind: string): void {
    const id = shown.get(kind)
    if (id === undefined) return
    toast.remove({ id })
    shown.delete(kind)
  }

  function show(kind: string, message: AppNotice): void {
    clear(kind)
    shown.set(
      kind,
      toast.add({ group: NOTICE_GROUP, closable: true, ...message, life: autoLife(message) }),
    )
  }

  return { show, clear }
}

// autoLife is a shown message's life: none for an error or one with a copy
// or action button, so it stays until closed or replaced; NOTICE_LIFE
// otherwise. A caller's own life is dropped.
function autoLife(message: AppNotice): number | undefined {
  if (message.severity === 'error') return undefined
  if (message.data?.copyText || message.data?.actions) return undefined
  return NOTICE_LIFE
}
