import type { ToastMessageOptions } from 'openvue/toast'
import { useToast } from 'openvue/usetoast'
import type { CoreError } from '../composables/useCoreCall'

// NOTICE_GROUP is the group of the app's one notice area (NoticeArea.vue);
// every notice is added to it.
export const NOTICE_GROUP = 'app'

// NoticeAction is a notice's recovery button. Its click leaves the notice
// open: what it starts replaces the notice (a fresh job, through
// useJobNotices).
export interface NoticeAction {
  label: string
  run: () => void
}

// AppNotice is a notice with the extras NoticeArea renders: copyText adds a
// button copying it, actions add recovery buttons, error renders the body
// as an ErrorBanner (with git's command and stderr behind its details
// toggle) instead of the summary.
export interface AppNotice extends ToastMessageOptions {
  data?: { copyText?: string; actions?: NoticeAction[]; error?: CoreError }
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
    shown.set(kind, toast.add({ group: NOTICE_GROUP, closable: true, ...message }))
  }

  return { show, clear }
}
