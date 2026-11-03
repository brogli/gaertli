import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { ChangeScope, type Refs } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { onRepoChanged } from '@/lib/windowEvents'

// refs holds the repo's branches, remotes, and tags, reloaded only when a
// ref moved (the "refs" scope).
export const useRefsStore = defineStore('refs', () => {
  const refs = ref<Refs | null>(null)
  // current is this worktree's checked-out branch, undefined while detached.
  const current = computed(() => refs.value?.branches?.find((b) => b.current))
  let wt = ''
  let unlisten: (() => void)[] = []

  async function refresh(): Promise<void> {
    refs.value = await Facade.Refs(wt)
  }

  async function load(id: string): Promise<void> {
    wt = id

    unlisten.forEach((off) => off())
    unlisten = [onRepoChanged([ChangeScope.RefsChanged], () => void refresh())]

    await refresh()
  }

  return { refs, current, load }
})
