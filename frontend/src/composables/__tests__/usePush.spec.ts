import { flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({ Facade: { Push: vi.fn(), PushTarget: vi.fn() } }))
vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))

import { usePush } from '../usePush'
import { useCoreCall } from '../useCoreCall'
import { useRefsStore } from '@/stores/refs'
import { branchFixture } from '@/testing/fixtures/branch'

function setBranch(overrides: Parameters<typeof branchFixture>[0] = {}): void {
  useRefsStore().refs = { branches: [branchFixture(overrides)], remotes: [], tags: [] }
}

describe('usePush', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.Push.mockReset().mockResolvedValue('job-1')
    Facade.PushTarget.mockReset()
  })

  it('FR-SYNC-03 no upstream, PushTarget auto with a remote: pushes the named branch there without asking', async () => {
    setBranch({ upstream: '' })
    Facade.PushTarget.mockResolvedValue({
      remote: 'origin',
      autoSetUpstream: true,
      branch: 'feature',
      hasUpstream: false,
    })
    const { push, upstreamQuestion } = usePush('/repo', useCoreCall().run)

    push()
    await flushPromises()

    expect(Facade.Push).toHaveBeenCalledWith('/repo', 'origin', 'feature')
    expect(upstreamQuestion.value).toBeNull()
  })

  it('FR-SYNC-03 no upstream, PushTarget not auto, then auto without a remote: asks both times naming PushTarget’s branch, never pushes', async () => {
    setBranch({ name: 'feature', upstream: '' })
    Facade.PushTarget.mockResolvedValueOnce({
      remote: 'origin',
      autoSetUpstream: false,
      branch: 'feature',
      hasUpstream: false,
    })
    const { push, upstreamQuestion } = usePush('/repo', useCoreCall().run)

    push()
    await flushPromises()
    expect(upstreamQuestion.value).toEqual({ branch: 'feature', remote: 'origin' })

    Facade.PushTarget.mockResolvedValueOnce({
      remote: '',
      autoSetUpstream: true,
      branch: 'feature',
      hasUpstream: false,
    })
    push()
    await flushPromises()
    expect(upstreamQuestion.value).toEqual({ branch: 'feature', remote: '' })

    expect(Facade.Push).not.toHaveBeenCalled()
  })

  it('FR-SYNC-03 a store without an upstream but PushTarget.hasUpstream true (a config-only change the store has not reloaded): pushes plainly', async () => {
    setBranch({ upstream: '' })
    Facade.PushTarget.mockResolvedValue({
      remote: 'origin',
      autoSetUpstream: false,
      branch: 'main',
      hasUpstream: true,
    })
    const { push, upstreamQuestion } = usePush('/repo', useCoreCall().run)

    push()
    await flushPromises()

    expect(Facade.Push).toHaveBeenCalledWith('/repo', '', '')
    expect(upstreamQuestion.value).toBeNull()
  })

  it('FR-SYNC-03 a branch with an upstream still calls PushTarget: it alone decides', async () => {
    setBranch({ upstream: 'origin/main' })
    Facade.PushTarget.mockResolvedValue({
      remote: 'origin',
      autoSetUpstream: false,
      branch: 'main',
      hasUpstream: true,
    })
    const { push } = usePush('/repo', useCoreCall().run)

    push()
    await flushPromises()

    expect(Facade.PushTarget).toHaveBeenCalledWith('/repo')
    expect(Facade.Push).toHaveBeenCalledWith('/repo', '', '')
  })
})
