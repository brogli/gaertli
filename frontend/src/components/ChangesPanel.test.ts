import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockStageFile, mockUnstageFile, mockStageAll, mockUnstageAll, mockStatus, mockOpenFile } = vi.hoisted(
  () => ({
    mockStageFile: vi.fn(),
    mockUnstageFile: vi.fn(),
    mockStageAll: vi.fn(),
    mockUnstageAll: vi.fn(),
    mockStatus: vi.fn(),
    mockOpenFile: vi.fn(),
  }),
)

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: {
    StageFile: mockStageFile,
    UnstageFile: mockUnstageFile,
    StageAll: mockStageAll,
    UnstageAll: mockUnstageAll,
    Status: mockStatus,
    OpenFile: mockOpenFile,
  },
}))

import ChangesPanel from './ChangesPanel.vue'
import { useChangesStore } from '../stores/changes'
import { changedFilesFixture } from '../testing/fixtures/changes'

const props = { wt: '/repo' }
const empty = changedFilesFixture()

describe('ChangesPanel', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockStageFile.mockReset().mockResolvedValue(undefined)
    mockUnstageFile.mockReset().mockResolvedValue(undefined)
    mockStageAll.mockReset().mockResolvedValue(undefined)
    mockUnstageAll.mockReset().mockResolvedValue(undefined)
    mockStatus.mockReset().mockResolvedValue(empty)
    mockOpenFile.mockReset().mockResolvedValue(undefined)
  })

  it('FR-CHG-02 clicking an unstaged row\'s Stage button calls Facade.StageFile with its path', async () => {
    const changes = useChangesStore()
    changes.files = { ...empty, unstaged: [{ path: 'a.txt', status: 'M' }] }

    const wrapper = mount(ChangesPanel, { props })
    await wrapper.get('[data-test="action"]').trigger('click')

    expect(mockStageFile).toHaveBeenCalledTimes(1)
    expect(mockStageFile).toHaveBeenCalledWith('/repo', 'a.txt')
  })

  it('FR-CHG-02 clicking a staged row\'s Unstage button calls Facade.UnstageFile with its path', async () => {
    const changes = useChangesStore()
    changes.files = { ...empty, staged: [{ path: 'a.txt', status: 'M' }] }

    const wrapper = mount(ChangesPanel, { props })
    await wrapper.get('[data-test="action"]').trigger('click')

    expect(mockUnstageFile).toHaveBeenCalledTimes(1)
    expect(mockUnstageFile).toHaveBeenCalledWith('/repo', 'a.txt')
  })

  it('FR-CHG-04 clicking an unstaged row (not its button) selects it as unstaged', async () => {
    const changes = useChangesStore()
    changes.files = { ...empty, unstaged: [{ path: 'a.txt', status: 'M' }] }

    const wrapper = mount(ChangesPanel, { props })
    await wrapper.get('li').trigger('click')

    expect(changes.selectedFile).toEqual({ path: 'a.txt', oldPath: '', staged: false })
  })

  it('FR-ED-02 clicking a row\'s "Open in editor" control calls Facade.OpenFile, not select', async () => {
    const changes = useChangesStore()
    changes.files = { ...empty, unstaged: [{ path: 'a.txt', status: 'M' }] }

    const wrapper = mount(ChangesPanel, { props })
    await wrapper.get('[data-test="open-editor"]').trigger('click')

    expect(mockOpenFile).toHaveBeenCalledWith('/repo', 'a.txt', 0)
    expect(changes.selectedFile).toBeNull()
  })

  it('FR-CHG-02 "Stage all" calls Facade.StageAll once; the index event reloads the list', async () => {
    const changes = useChangesStore()
    changes.files = { ...empty, unstaged: [{ path: 'a.txt', status: 'M' }] }

    const wrapper = mount(ChangesPanel, { props })
    const stageAll = wrapper.findAll('button').find((b) => b.text() === 'Stage all')
    await stageAll?.trigger('click')
    await flushPromises()

    expect(mockStageAll).toHaveBeenCalledWith('/repo')
    expect(mockStatus).not.toHaveBeenCalled()
  })
})
