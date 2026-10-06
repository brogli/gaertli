import { computed, reactive, type ComputedRef } from 'vue'
import type { LineRange } from '@/lib/conflictRegions'

// MergeDraft is a merge result with unsaved edits: its text, the conflict
// regions in it, and the fingerprint of the working file it was read from.
export interface MergeDraft {
  text: string
  fingerprint: string
  regions: LineRange[]
}

// The window's drafts, keyed by worktree and path. They live as long as the
// window's page.
const drafts = reactive(new Map<string, MergeDraft>())

const keyOf = (wt: string, path: string) => `${wt}\n${path}`

interface MergeDrafts {
  get: (wt: string, path: string) => MergeDraft | undefined
  set: (wt: string, path: string, draft: MergeDraft) => void
  discard: (wt: string, path: string) => void
  keepOnly: (wt: string, paths: string[]) => void
  clearAll: () => void
  hasUnsaved: ComputedRef<boolean>
}

// useMergeDrafts keeps the merge results with unsaved edits, so showing
// another file loses nothing.
export function useMergeDrafts(): MergeDrafts {
  return {
    get: (wt: string, path: string) => drafts.get(keyOf(wt, path)),
    set: (wt, path, draft) => void drafts.set(keyOf(wt, path), draft),
    discard: (wt, path) => void drafts.delete(keyOf(wt, path)),
    keepOnly: (wt, paths) => {
      for (const key of drafts.keys()) {
        if (key.startsWith(`${wt}\n`) && !paths.some((path) => key === keyOf(wt, path)))
          drafts.delete(key)
      }
    },
    clearAll: () => drafts.clear(),
    hasUnsaved: computed(() => drafts.size > 0),
  }
}
