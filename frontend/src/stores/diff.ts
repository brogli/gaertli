import { defineStore } from 'pinia'
import { ref } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type {
  DiffFile,
  DiffHunk,
  DiffOptions,
} from '../../bindings/github.com/brogli/nebegit/internal/core'
import { DiffKind, type DiffTarget } from '../lib/diffTarget'

// diff holds the currently viewed file's diff, cached per path for the
// commit being viewed; the cache empties when the commit or the worktree
// changes.
export const useDiffStore = defineStore('diff', () => {
  const file = ref<DiffFile | null>(null)
  const loading = ref(false)

  const cache = new Map<string, DiffFile>()
  let cachedScope = ''
  let lastFetch: (() => Promise<DiffFile>) | null = null
  let lastKey = ''

  // loadInto loads file through fetch, cached by key and whether whitespace
  // is ignored. The cache clears when wt or sha changes: sha is a commit, a
  // range's "a..b", "<sha>..working" or "working". key is the path, plus
  // staged for a working-tree diff, since one path can have a staged and an
  // unstaged diff at once.
  async function loadInto(
    wt: string,
    sha: string,
    key: string,
    opts: DiffOptions,
    fetch: () => Promise<DiffFile>,
  ): Promise<void> {
    const scope = `${wt}\n${sha}`
    if (scope !== cachedScope) {
      cache.clear()
      cachedScope = scope
    }
    const cacheKey = `${key}:${opts.ignoreWhitespace}`
    // refetch reloads what is shown, whether it was fetched or cached.
    lastFetch = fetch
    lastKey = cacheKey
    const cached = opts.force ? undefined : cache.get(cacheKey)
    if (cached) {
      latest++
      loading.value = false
      show(cacheKey, cached)
      return
    }
    loading.value = true
    await fetchAndShow(cacheKey, fetch)
  }

  // latest numbers every load and refetch; only the latest one's answer is
  // shown. A slower answer for a file left meanwhile is dropped, its
  // failure too.
  let latest = 0

  async function fetchAndShow(key: string, fetch: () => Promise<DiffFile>): Promise<void> {
    const request = ++latest
    try {
      const result = await fetch()
      if (request === latest) show(key, result)
    } catch (e) {
      if (request === latest) throw e
    } finally {
      if (request === latest) loading.value = false
    }
  }

  // show caches result and displays it, keeping the displayed object when
  // the diff is unchanged so the view isn't rebuilt for nothing.
  function show(key: string, result: DiffFile): void {
    cache.set(key, result)
    const f = file.value
    const same =
      f &&
      f.path === result.path &&
      f.oldPath === result.oldPath &&
      f.binary === result.binary &&
      f.tooLarge === result.tooLarge &&
      f.oldContent === result.oldContent &&
      f.newContent === result.newContent &&
      sameHunks(f.hunks, result.hunks)
    if (!same) file.value = result
  }

  // refetch re-runs the last load past the cache, with the same arguments
  // (incl. an explicit "load anyway"): the working tree changed under it.
  async function refetch(): Promise<void> {
    if (lastFetch) await fetchAndShow(lastKey, lastFetch)
  }

  // load loads target's file: a commit's change, a range's, a commit against
  // the working tree, or a working-tree change on its staged or unstaged side.
  async function load(wt: string, target: DiffTarget, opts: DiffOptions): Promise<void> {
    const { path, oldPath = '', sha = '', from = '', to = '', staged = false } = target
    switch (target.kind) {
      case DiffKind.CommitDiff:
        return loadInto(wt, sha, path, opts, () => Facade.DiffFile(wt, sha, path, oldPath, opts))
      case DiffKind.RangeDiff:
        return loadInto(wt, `${from}..${to}`, path, opts, () =>
          Facade.DiffRange(wt, from, to, path, oldPath, opts),
        )
      case DiffKind.AgainstWorkingDiff:
        return loadInto(wt, `${sha}..working`, path, opts, () =>
          Facade.DiffAgainstWorking(wt, sha, path, oldPath, opts),
        )
      default:
        return loadInto(wt, 'working', `${staged}:${path}`, opts, () =>
          Facade.DiffWorking(wt, path, oldPath, staged, opts),
        )
    }
  }

  return { file, loading, load, refetch }
})

function sameHunks(a: DiffHunk[] | null | undefined, b: DiffHunk[] | null | undefined): boolean {
  return JSON.stringify(a ?? []) === JSON.stringify(b ?? [])
}
