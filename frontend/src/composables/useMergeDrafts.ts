import { reactive, shallowReactive } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { ResultRegion } from '@/lib/conflictRegions'
import { toCoreError, type CoreError } from './useCoreCall'

// MergeDraft is a merge result with unsaved edits: its text, the conflict
// regions in it with their states, the fingerprint of the working file it
// was read from and the conflictId of the conflict it was made for.
export interface MergeDraft {
  text: string
  fingerprint: string
  conflictId: string
  regions: ResultRegion[]
}

// ReadDraft reads a draft's content from where it is kept, an editor or a
// value.
type ReadDraft = () => Omit<MergeDraft, 'conflictId'>

// Kept is a draft as the page keeps it: read only when it is written or
// asked for, not on each edit.
interface Kept {
  conflictId: string
  read: ReadDraft
}

const draftOf = ({ conflictId, read }: Kept): MergeDraft => ({ ...read(), conflictId })

// The page's copy of the drafts, keyed by worktree and path. The core's
// worktree session holds them, so they outlive the window; this copy is what
// the editor reads synchronously.
const drafts = shallowReactive(new Map<string, Kept>())

// writeErrors holds, per draft, why its last write to the core failed.
const writeErrors = reactive(new Map<string, CoreError>())

const WRITE_DELAY_MS = 500

// A write goes out at once and opens a pause of WRITE_DELAY_MS; changes
// within the pause are queued for one write as it ends.
interface Pause {
  wt: string
  path: string
  timer: ReturnType<typeof setTimeout>
  queued: boolean
}
const pauses = new Map<string, Pause>()

const keyOf = (wt: string, path: string) => `${wt}\n${path}`

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
window.addEventListener('pagehide', () => {
  for (const key of pauses.keys()) flushQueued(key)
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

// Each draft belongs to the conflict it was made for: a draft of an earlier
// conflict in the path counts as none.
interface MergeDrafts {
  has: (wt: string, path: string, conflictId: string) => boolean
  get: (wt: string, path: string, conflictId: string) => MergeDraft | undefined
  writeError: (wt: string, path: string) => CoreError | undefined
  load: (wt: string, path: string, conflictId: string) => Promise<void>
  set: (wt: string, path: string, conflictId: string, read: ReadDraft) => void
  flush: (wt: string, path: string) => void
  discard: (wt: string, path: string) => Promise<void>
  keepOnly: (wt: string, paths: string[]) => void
}

// useMergeDrafts keeps the merge results with unsaved edits, so showing
// another file or reopening the diff window loses nothing.
export function useMergeDrafts(): MergeDrafts {
  return {
    has: (wt, path, conflictId) => !!keptFor(wt, path, conflictId),
    get: (wt, path, conflictId) => {
      const kept = keptFor(wt, path, conflictId)
      return kept && draftOf(kept)
    },
    writeError: (wt, path) => writeErrors.get(keyOf(wt, path)),
    // load fetches the core's draft unless the page already holds one for
    // conflictId; it drops one of an earlier conflict.
    load: async (wt, path, conflictId) => {
      const key = keyOf(wt, path)
      if (keptFor(wt, path, conflictId)) return
      drop(key)
      const stored = await Facade.MergeDraft(wt, path)
      if (stored?.conflictId !== conflictId || drafts.has(key)) return
      const content = {
        text: stored.text,
        fingerprint: stored.fingerprint,
        regions: stored.regions ?? [],
      }
      drafts.set(key, { conflictId, read: () => content })
    },
    // set keeps the draft read reads, made for conflictId, and sends it to
    // the core, coalescing a burst of edits into one trailing write.
    set: (wt, path, conflictId, read) => {
      drafts.set(keyOf(wt, path), { conflictId, read })
      writeSoon(wt, path)
    },
    // flush sends a write still waiting.
    flush: (wt, path) => flushQueued(keyOf(wt, path)),
    // discard drops the draft, resolving once its write to the core ended.
    discard: (wt, path) => {
      drafts.delete(keyOf(wt, path))
      cancelWrites(keyOf(wt, path))
      return writeToCore(wt, path)
    },
    keepOnly: (wt, paths) => {
      for (const key of drafts.keys()) {
        if (!key.startsWith(`${wt}\n`) || paths.some((path) => key === keyOf(wt, path))) continue
        drop(key)
      }
    },
  }
}
