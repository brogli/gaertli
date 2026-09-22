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

  // loadInto shares load's and loadRange's cache logic. sha is what the
  // cache is keyed on: a single commit, or a range's "a..b".
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

  // loadWorking is DiffFile's counterpart for a working-tree change
  // (FR-CHG-04), keyed on the literal string "working" — there is no
  // commit/range to key by, and a stage/unstage action reloads the
  // changes panel anyway, so a coarser cache key costs nothing observable.
  async function loadWorking(
    wt: string,
    path: string,
    oldPath: string,
    staged: boolean,
    force = false,
  ): Promise<void> {
    return loadInto('working', path, force, () => Facade.DiffWorking(wt, path, oldPath, staged, force))
  }

  return { file, loading, load, loadRange, loadWorking }
})
