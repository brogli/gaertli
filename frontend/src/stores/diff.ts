import { defineStore } from 'pinia'
import { ref } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { DiffFile } from '../../bindings/github.com/brogli/nebegit/internal/core'

// diff holds the currently viewed file's two-sided content (FR-LOG-04,
// ADR-7), cached per path for the commit being viewed so stepping between
// its files is free; the cache empties when the commit changes, so it
// never outgrows one commit's files.
export const useDiffStore = defineStore('diff', () => {
  const file = ref<DiffFile | null>(null)
  const loading = ref(false)

  const cache = new Map<string, DiffFile>()
  let cachedSha = ''

  // loadInto shares DiffFile's and DiffRange's cache logic (spec 21): sha
  // is the commit being viewed for cache-invalidation purposes — either a
  // single sha, or a range's "a..b" (a range is "the commit being viewed"
  // for cache purposes).
  async function loadInto(
    sha: string,
    path: string,
    force: boolean,
    fetch: () => Promise<DiffFile>,
  ): Promise<void> {
    if (sha !== cachedSha) {
      cache.clear()
      cachedSha = sha
    }
    if (!force) {
      const cached = cache.get(path)
      if (cached) {
        file.value = cached
        return
      }
    }
    loading.value = true
    try {
      const result = await fetch()
      cache.set(path, result)
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

  // loadRange is DiffFile's counterpart for two arbitrary commits (FR-CMP-02).
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

  return { file, loading, load, loadRange }
})
