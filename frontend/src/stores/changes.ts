import { defineStore } from 'pinia'
import { ref } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { ChangedFiles } from '../../bindings/github.com/brogli/nebegit/internal/core'

// changes holds a worktree's current uncommitted changes, kept current by
// the repo:changed{scope:"index"} listener of the view showing them
// (WorktreeContent.vue, DiffWindowView.vue).
export const useChangesStore = defineStore('changes', () => {
  const files = ref<ChangedFiles | null>(null)
  const loading = ref(false)

  async function load(wt: string): Promise<void> {
    loading.value = true
    try {
      files.value = await Facade.Status(wt)
    } finally {
      loading.value = false
    }
  }

  return { files, loading, load }
})
