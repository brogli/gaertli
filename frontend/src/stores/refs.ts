import { defineStore } from 'pinia'
import { ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { Refs } from '../../bindings/github.com/brogli/nebegit/internal/core'

// refs holds the sidebar's branches, remotes, and tags, reloaded only when
// a ref actually moved (the "refs" scope) — unlike repo/log, a state-only
// change such as an in-progress rebase leaves refs untouched.
export const useRefsStore = defineStore('refs', () => {
  const refs = ref<Refs | null>(null)
  let wt = ''
  let unlistenChanged: (() => void) | undefined
  let unlistenReconnected: (() => void) | undefined

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

    // the window's distro core respawned: its data starts over, as on mount
    unlistenReconnected?.()
    unlistenReconnected = Events.On('core:reconnected', () => void refresh())

    await refresh()
  }

  return { refs, load }
})
