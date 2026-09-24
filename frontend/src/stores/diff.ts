import { defineStore } from 'pinia'
import { ref } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type {
  DiffFile,
  DiffHunk,
  DiffOptions,
} from '../../bindings/github.com/brogli/nebegit/internal/core'

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
  // range's "a..b", or "working". key is the path, plus staged for a
  // working-tree diff, since one path can have a staged and an unstaged diff
  // at once.
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
    if (!opts.force) {
      const cached = cache.get(cacheKey)
      if (cached) {
        file.value = cached
        return
      }
    }
    lastFetch = fetch
    lastKey = cacheKey
    loading.value = true
    try {
      show(cacheKey, await fetch())
    } finally {
      loading.value = false
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
    if (lastFetch) show(lastKey, await lastFetch())
  }

  async function load(
    wt: string,
    sha: string,
    path: string,
    oldPath: string,
    opts: DiffOptions,
  ): Promise<void> {
    return loadInto(wt, sha, path, opts, () => Facade.DiffFile(wt, sha, path, oldPath, opts))
  }

  // loadRange is load's counterpart for two arbitrary commits.
  async function loadRange(
    wt: string,
    a: string,
    b: string,
    path: string,
    oldPath: string,
    opts: DiffOptions,
  ): Promise<void> {
    return loadInto(wt, `${a}..${b}`, path, opts, () =>
      Facade.DiffRange(wt, a, b, path, oldPath, opts),
    )
  }

  // loadWorking is load's counterpart for a working-tree change.
  async function loadWorking(
    wt: string,
    path: string,
    oldPath: string,
    staged: boolean,
    opts: DiffOptions,
  ): Promise<void> {
    return loadInto(wt, 'working', `${staged}:${path}`, opts, () =>
      Facade.DiffWorking(wt, path, oldPath, staged, opts),
    )
  }

  // loadAgainstWorking is load's counterpart for a commit compared against
  // the current working tree.
  async function loadAgainstWorking(
    wt: string,
    sha: string,
    path: string,
    oldPath: string,
    opts: DiffOptions,
  ): Promise<void> {
    return loadInto(wt, `${sha}..working`, path, opts, () =>
      Facade.DiffAgainstWorking(wt, sha, path, oldPath, opts),
    )
  }

  return { file, loading, load, loadRange, loadWorking, loadAgainstWorking, refetch }
})

function sameHunks(a: DiffHunk[] | null | undefined, b: DiffHunk[] | null | undefined): boolean {
  return JSON.stringify(a ?? []) === JSON.stringify(b ?? [])
}
