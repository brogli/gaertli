import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({ Facade: { Status: vi.fn() } }))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

import { useChangesStore } from './changes'
import { changedFilesFixture } from '../testing/fixtures/changes'

describe('changes store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.Status.mockReset()
  })

  it('FR-CHG-01 load(wt) fills files from Status and clears loading once done', async () => {
    const result = changedFilesFixture({ staged: [{ path: 'a.txt', status: 'M' }] })
    Facade.Status.mockResolvedValue(result)
    const changes = useChangesStore()

    const promise = changes.load('/repo')
    expect(changes.loading).toBe(true)
    await promise

    expect(Facade.Status).toHaveBeenCalledWith('/repo')
    expect(changes.files).toEqual(result)
    expect(changes.loading).toBe(false)
  })
})
