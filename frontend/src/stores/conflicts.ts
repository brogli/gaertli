import { defineStore } from 'pinia'
import { onScopeDispose, reactive, shallowReactive } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/gaertli/internal/app'
import type {
  DraftChange,
  MergeDraft,
} from '../../bindings/github.com/brogli/gaertli/internal/core'
import { toCoreError, type CoreError } from '@/composables/useCoreCall'

// ReadDraft reads a draft's content from where it is kept, an editor or a
// value.
type ReadDraft = () => Omit<MergeDraft, 'conflictId'>

// Draft is a MergeDraft whose changes are always a list.
type Draft = MergeDraft & { changes: DraftChange[] }

// Kept is a draft as the page keeps it: read only when it is written or
// asked for, not on each edit.
interface Kept {
  conflictId: string
  read: ReadDraft
}

const draftOf = ({ conflictId, read }: Kept): Draft => {
  const content = read()
  return { ...content, changes: content.changes ?? [], conflictId }
}

const WRITE_DELAY_MS = 500

// A write goes out at once and opens a pause of WRITE_DELAY_MS; changes
// within the pause are queued for one write as it ends.
interface Pause {
  wt: string
  path: string
  timer: ReturnType<typeof setTimeout>
  queued: boolean
}

const keyOf = (wt: string, path: string): string => `${wt}\n${path}`

// conflicts keeps the merge drafts, so showing another file or reopening the
// diff window loses nothing. Each draft belongs to the conflict it was made
// for: a draft of an earlier conflict in the path counts as none.
export const useConflictsStore = defineStore('conflicts', () => {
  // The page's copy of the drafts, keyed by worktree and path. The core's
  // worktree session holds them, so they outlive the window; this copy is what
  // the editor reads synchronously.
  const drafts = shallowReactive(new Map<string, Kept>())

  // writeErrors holds, per draft, why its last write to the core failed.
  const writeErrors = reactive(new Map<string, CoreError>())

  const pauses = new Map<string, Pause>()

  function cancelWrites(key: string): void {
    clearTimeout(pauses.get(key)?.timer)
    pauses.delete(key)
  }

  async function writeToCore(wt: string, path: string): Promise<void> {
    const key = keyOf(wt, path)
    try {
      const kept = drafts.get(key)
      await Facade.SetMergeDraft(wt, path, kept ? draftOf(kept) : null)
      writeErrors.delete(key)
    } catch (e) {
      writeErrors.set(key, toCoreError(e))
    }
  }

  function writeSoon(wt: string, path: string): void {
    const key = keyOf(wt, path)
    const paused = pauses.get(key)
    if (paused) {
      paused.queued = true
      return
    }
    void writeToCore(wt, path)
    const pause: Pause = {
      wt,
      path,
      queued: false,
      timer: setTimeout(() => {
        pauses.delete(key)
        if (pause.queued) writeSoon(wt, path)
      }, WRITE_DELAY_MS),
    }
    pauses.set(key, pause)
  }

  function flushQueued(key: string): void {
    const pause = pauses.get(key)
    if (!pause?.queued) return
    cancelWrites(key)
    void writeToCore(pause.wt, pause.path)
  }

  // A closing window's page does not unmount its editors: the writes still
  // waiting go out as the page hides.
  const flushAll = (): void => {
    for (const key of pauses.keys()) flushQueued(key)
  }
  window.addEventListener('pagehide', flushAll)
  onScopeDispose(() => {
    window.removeEventListener('pagehide', flushAll)
    for (const key of pauses.keys()) cancelWrites(key)
  })

  // drop forgets the page's draft and its writes still waiting.
  function drop(key: string): void {
    drafts.delete(key)
    writeErrors.delete(key)
    cancelWrites(key)
  }

  // keptFor is the page's draft of path made for conflictId.
  function keptFor(wt: string, path: string, conflictId: string): Kept | undefined {
    const kept = drafts.get(keyOf(wt, path))
    return kept?.conflictId === conflictId ? kept : undefined
  }

  function has(wt: string, path: string, conflictId: string): boolean {
    return !!keptFor(wt, path, conflictId)
  }

  function get(wt: string, path: string, conflictId: string): Draft | undefined {
    const kept = keptFor(wt, path, conflictId)
    return kept && draftOf(kept)
  }

  function writeError(wt: string, path: string): CoreError | undefined {
    return writeErrors.get(keyOf(wt, path))
  }

  // load fetches the core's draft unless the page already holds one for
  // conflictId; it drops one of an earlier conflict.
  async function load(wt: string, path: string, conflictId: string): Promise<void> {
    const key = keyOf(wt, path)
    if (keptFor(wt, path, conflictId)) return
    drop(key)
    const stored = await Facade.MergeDraft(wt, path)
    if (stored?.conflictId !== conflictId || drafts.has(key)) return
    const { text, fingerprint, changes } = stored
    drafts.set(key, { conflictId, read: () => ({ text, fingerprint, changes }) })
  }

  // set keeps the draft read reads, made for conflictId, and sends it to the
  // core, coalescing a burst of edits into one trailing write.
  function set(wt: string, path: string, conflictId: string, read: ReadDraft): void {
    drafts.set(keyOf(wt, path), { conflictId, read })
    writeSoon(wt, path)
  }

  // flush sends a write still waiting.
  function flush(wt: string, path: string): void {
    flushQueued(keyOf(wt, path))
  }

  // discard drops the draft, resolving once its write to the core ended.
  function discard(wt: string, path: string): Promise<void> {
    drafts.delete(keyOf(wt, path))
    cancelWrites(keyOf(wt, path))
    return writeToCore(wt, path)
  }

  function keepOnly(wt: string, paths: string[]): void {
    for (const key of drafts.keys()) {
      if (!key.startsWith(`${wt}\n`) || paths.some((path) => key === keyOf(wt, path))) continue
      drop(key)
    }
  }

  return { has, get, writeError, load, set, flush, discard, keepOnly }
})
