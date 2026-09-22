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

  // loadInto shares load's and loadRange's cache logic. sha is what the
  // cache clears on: a single commit, or a range's "a..b". key is the
  // cache slot within that sha; load/loadRange key on path alone,
  // loadWorking also keys on staged since the same path has a staged and
  // an unstaged diff at once.
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
    loading.value = true
    try {
      const result = await fetch()
      cache.set(key, result)
      file.value = result
    } finally {
      loading.value = false
    }
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

  // loadWorking is load's counterpart for a working-tree change, keyed on
  // the literal "working": there is no commit or range to key by.
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

  return { file, loading, load, loadRange, loadWorking }
})
