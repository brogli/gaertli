import { flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade, AppService } = vi.hoisted(() => ({
  Facade: { Summary: vi.fn() },
  AppService: { UpdateWindowTitle: vi.fn() },
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade, AppService }))

vi.mock('@wailsio/runtime', () => import('../testing/runtime'))
import { Events, emit } from '../testing/runtime'

import { useRepoStore } from './repo'
import { summaryFixture } from '../testing/fixtures/summary'

describe('repo store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.Summary.mockReset().mockResolvedValue(summaryFixture())
    Events.On.mockReset()
    AppService.UpdateWindowTitle.mockReset().mockResolvedValue(undefined)
  })

  it("FR-WT-09 calls UpdateWindowTitle with the worktree's path and branch after loading Summary", async () => {
    const repo = useRepoStore()

    await repo.loadSummary('/home/user/project')

    expect(AppService.UpdateWindowTitle).toHaveBeenCalledTimes(1)
    expect(AppService.UpdateWindowTitle).toHaveBeenCalledWith('/home/user/project', 'main')
  })

  it('FR-REPO-04 reloads Summary on a state or refs repo:changed event, not on index', async () => {
    const repo = useRepoStore()
    await repo.loadSummary('/repo')
    Facade.Summary.mockClear()

    emit('repo:changed', { data: { scope: 'state' } })
    emit('repo:changed', { data: { scope: 'refs' } })
    await flushPromises()

    expect(Facade.Summary).toHaveBeenCalledTimes(2)

    emit('repo:changed', { data: { scope: 'index' } })
    await flushPromises()

    expect(Facade.Summary).toHaveBeenCalledTimes(2)
  })

  it('ADR-4 reloads Summary on a core:reconnected event', async () => {
    const repo = useRepoStore()
    await repo.loadSummary('/repo')
    Facade.Summary.mockClear()

    emit('core:reconnected')
    await flushPromises()

    expect(Facade.Summary).toHaveBeenCalledTimes(1)
  })
})
