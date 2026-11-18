// Requirements: FR-CHG-01, FR-REPO-04
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({ Facade: { Status: vi.fn() } }))

vi.mock('../../../bindings/github.com/brogli/gaertli/internal/app', () => ({ Facade }))

import { useChangesStore } from '../changes'
import { changedFilesFixture } from '@/testing/fixtures/changes'

describe('changes store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.Status.mockReset()
  })

  it('load(wt) fills files from Status and clears loading once done', async () => {
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

  it('loading stays true until the last of overlapping loads ends', async () => {
    let endFirst!: (files: unknown) => void
    Facade.Status.mockReturnValueOnce(new Promise((resolve) => (endFirst = resolve)))
    let endSecond!: (files: unknown) => void
    Facade.Status.mockReturnValueOnce(new Promise((resolve) => (endSecond = resolve)))
    const changes = useChangesStore()

    const first = changes.load('/repo')
    const second = changes.load('/repo')
    endFirst(changedFilesFixture())
    await first
    expect(changes.loading).toBe(true)

    endSecond(changedFilesFixture())
    await second
    expect(changes.loading).toBe(false)
  })
})
