import { defineStore } from 'pinia'
import { ref } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { ChangedFile, ChangedFiles } from '../../bindings/github.com/brogli/nebegit/internal/core'

// changes holds a worktree's current uncommitted changes and the file
// selected for the diff pane, kept current by the repo:changed{scope:
// "index"} listener in WorktreeContent.vue.
export const useChangesStore = defineStore('changes', () => {
  const files = ref<ChangedFiles | null>(null)
  const loading = ref(false)
  const selectedFile = ref<{ path: string; oldPath: string; staged: boolean; status: string } | null>(null)

  async function load(wt: string): Promise<void> {
    loading.value = true
    try {
      files.value = await Facade.Status(wt)
    } finally {
      loading.value = false
    }
  }

  function selectFile(file: ChangedFile, staged: boolean): void {
    selectedFile.value = { path: file.path, oldPath: file.oldPath ?? '', staged, status: file.status }
  }

  function clearSelection(): void {
    selectedFile.value = null
  }

  return { files, loading, selectedFile, load, selectFile, clearSelection }
})
