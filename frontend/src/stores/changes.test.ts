import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockStatus } = vi.hoisted(() => ({ mockStatus: vi.fn() }))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { Status: mockStatus },
}))

import { useChangesStore } from './changes'

describe('changes store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockStatus.mockReset()
  })

  it('FR-CHG-01 load(wt) fills files from Status and clears loading once done', async () => {
    const result = {
      staged: [{ path: 'a.txt', status: 'M' }],
      unstaged: [],
      untracked: [],
      conflicted: [],
    }
    mockStatus.mockResolvedValue(result)
    const changes = useChangesStore()

    const promise = changes.load('/repo')
    expect(changes.loading).toBe(true)
    await promise

    expect(mockStatus).toHaveBeenCalledWith('/repo')
    expect(changes.files).toEqual(result)
    expect(changes.loading).toBe(false)
  })
})
