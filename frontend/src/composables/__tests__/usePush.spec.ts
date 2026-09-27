import { flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({ Facade: { Push: vi.fn(), PushTarget: vi.fn() } }))
vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

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

  it('FR-SYNC-03 no upstream, PushTarget auto with a remote: pushes there without asking', async () => {
    setBranch({ upstream: '' })
    Facade.PushTarget.mockResolvedValue({ remote: 'origin', autoSetUpstream: true })
    const { push, upstreamQuestion } = usePush('/repo', useCoreCall().run)

    push()
    await flushPromises()

    expect(Facade.Push).toHaveBeenCalledWith('/repo', 'origin')
    expect(upstreamQuestion.value).toBeNull()
  })

  it('FR-SYNC-03 no upstream, PushTarget not auto, then auto without a remote: asks both times, never pushes', async () => {
    setBranch({ name: 'feature', upstream: '' })
    Facade.PushTarget.mockResolvedValueOnce({ remote: 'origin', autoSetUpstream: false })
    const { push, upstreamQuestion } = usePush('/repo', useCoreCall().run)

    push()
    await flushPromises()
    expect(upstreamQuestion.value).toEqual({ branch: 'feature', remote: 'origin' })

    Facade.PushTarget.mockResolvedValueOnce({ remote: '', autoSetUpstream: true })
    push()
    await flushPromises()
    expect(upstreamQuestion.value).toEqual({ branch: 'feature', remote: '' })

    expect(Facade.Push).not.toHaveBeenCalled()
  })

  it('FR-SYNC-03 a branch with an upstream pushes plainly; PushTarget is never called', async () => {
    setBranch({ upstream: 'origin/main' })
    const { push } = usePush('/repo', useCoreCall().run)

    push()
    await flushPromises()

    expect(Facade.Push).toHaveBeenCalledWith('/repo', '')
    expect(Facade.PushTarget).not.toHaveBeenCalled()
  })
})
