import { defineStore } from 'pinia'
import { ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { Refs } from '../../bindings/github.com/brogli/nebegit/internal/core'

// refs holds the sidebar's branches, remotes, and tags (FR-LOG-01),
// reloaded only when a ref actually moved (spec 12's "refs" scope) —
// unlike repo/log, a state-only change (e.g. an in-progress rebase)
// leaves refs untouched.
export const useRefsStore = defineStore('refs', () => {
  const refs = ref<Refs | null>(null)
  let wt = ''
  let unlistenChanged: (() => void) | undefined

  async function refresh(): Promise<void> {
    refs.value = await Facade.Refs(wt)
  }

  async function load(id: string): Promise<void> {
    wt = id

    unlistenChanged?.()
    unlistenChanged = Events.On('repo:changed', (ev) => {
      const { scope } = ev.data as { scope: string }
      if (scope === 'refs') void refresh()
    })

    await refresh()
  }

  return { refs, load }
})
