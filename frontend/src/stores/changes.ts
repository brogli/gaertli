import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/gaertli/internal/app'
import type { ChangedFiles } from '../../bindings/github.com/brogli/gaertli/internal/core'

// changes holds a worktree's current uncommitted changes, kept current by
// the repo:changed{scope:"index"} listener of the view showing them
// (WorktreeContent.vue, DiffWindowView.vue), and the commit message being
// typed in the commit box.
export const useChangesStore = defineStore('changes', () => {
  const files = ref<ChangedFiles | null>(null)
  const loadsInFlight = ref(0)
  const loading = computed(() => loadsInFlight.value > 0)
  const messageDraft = ref('')

  async function load(wt: string): Promise<void> {
    loadsInFlight.value++
    try {
      files.value = await Facade.Status(wt)
    } finally {
      loadsInFlight.value--
    }
  }

  return { files, loading, messageDraft, load }
})
