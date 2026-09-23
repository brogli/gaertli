import { defineStore } from 'pinia'
import { ref } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { DiffFile } from '../../bindings/github.com/brogli/nebegit/internal/core'

// diff holds the currently viewed file's two-sided content, cached per path
// for the commit being viewed; the cache empties when the commit changes.
export const useDiffStore = defineStore('diff', () => {
  const file = ref<DiffFile | null>(null)
  const loading = ref(false)

  const cache = new Map<string, DiffFile>()
  let cachedSha = ''
  let lastFetch: (() => Promise<DiffFile>) | null = null
  let lastKey = ''

  // loadInto loads file through fetch, cached by key. The cache clears
  // when sha changes: a commit, a range's "a..b", or "working". key is the
  // path, plus staged for a working-tree diff, since one path can have a
  // staged and an unstaged diff at once.
  async function loadInto(
    sha: string,
    key: string,
    force: boolean,
    fetch: () => Promise<DiffFile>,
  ): Promise<void> {
    if (sha !== cachedSha) {
      cache.clear()
      cachedSha = sha
    }
    if (!force) {
      const cached = cache.get(key)
      if (cached) {
        file.value = cached
        return
      }
    }
    lastFetch = fetch
    lastKey = key
    loading.value = true
    try {
      show(key, await fetch())
    } finally {
      loading.value = false
    }
  }

  // show caches result and displays it, keeping the displayed object when
  // the content is unchanged so the view isn't rebuilt for nothing.
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
      f.newContent === result.newContent
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
    force = false,
  ): Promise<void> {
    return loadInto(sha, path, force, () => Facade.DiffFile(wt, sha, path, oldPath, force))
  }

  // loadRange is load's counterpart for two arbitrary commits.
  async function loadRange(
    wt: string,
    a: string,
    b: string,
    path: string,
    oldPath: string,
    force = false,
  ): Promise<void> {
    return loadInto(`${a}..${b}`, path, force, () => Facade.DiffRange(wt, a, b, path, oldPath, force))
  }

  // loadWorking is load's counterpart for a working-tree change.
  async function loadWorking(
    wt: string,
    path: string,
    oldPath: string,
    staged: boolean,
    force = false,
  ): Promise<void> {
    return loadInto('working', `${staged}:${path}`, force, () =>
      Facade.DiffWorking(wt, path, oldPath, staged, force),
    )
  }

  // loadAgainstWorking is load's counterpart for a commit compared against
  // the current working tree (compare.workingCommit).
  async function loadAgainstWorking(
    wt: string,
    sha: string,
    path: string,
    oldPath: string,
    force = false,
  ): Promise<void> {
    return loadInto(`${sha}..working`, path, force, () =>
      Facade.DiffAgainstWorking(wt, sha, path, oldPath, force),
    )
  }

  return { file, loading, load, loadRange, loadWorking, loadAgainstWorking, refetch }
})
