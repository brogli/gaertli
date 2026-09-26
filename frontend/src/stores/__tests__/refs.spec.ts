import { flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ChangeScope } from '../../../bindings/github.com/brogli/nebegit/internal/core'

const { Facade } = vi.hoisted(() => ({
  Facade: { Refs: vi.fn() },
}))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))
import { Events, emit } from '@/testing/runtime'

import { useRefsStore } from '../refs'

describe('refs store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.Refs.mockReset().mockResolvedValue({ branches: [], remotes: [], tags: [] })
    Events.On.mockReset()
  })

  it('FR-LOG-01 reloads Refs on a refs repo:changed event, not on state', async () => {
    const refs = useRefsStore()
    await refs.load('/repo')
    expect(Facade.Refs).toHaveBeenCalledTimes(1)

    emit('repo:changed', { data: { scope: ChangeScope.RefsChanged } })
    emit('repo:changed', { data: { scope: ChangeScope.StateChanged } })
    await flushPromises()

    expect(Facade.Refs).toHaveBeenCalledTimes(2)
  })
})
