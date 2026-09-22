import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockDiffWorking } = vi.hoisted(() => ({ mockDiffWorking: vi.fn() }))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { DiffWorking: mockDiffWorking },
}))

import { useDiffStore } from './diff'

describe('diff store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockDiffWorking.mockReset()
  })

  it('FR-CHG-04 loadWorking caches the staged and unstaged diff of the same path separately', async () => {
    const diff = useDiffStore()
    const staged = { path: 'a.txt', binary: false, oldContent: 'staged old', newContent: 'staged new' }
    const unstaged = { path: 'a.txt', binary: false, oldContent: 'unstaged old', newContent: 'unstaged new' }
    mockDiffWorking.mockResolvedValueOnce(staged).mockResolvedValueOnce(unstaged)

    await diff.loadWorking('/repo', 'a.txt', '', true)
    expect(diff.file).toEqual(staged)

    await diff.loadWorking('/repo', 'a.txt', '', false)
    expect(diff.file).toEqual(unstaged)

    await diff.loadWorking('/repo', 'a.txt', '', true)
    expect(diff.file).toEqual(staged)
    expect(mockDiffWorking).toHaveBeenCalledTimes(2)
  })
})
