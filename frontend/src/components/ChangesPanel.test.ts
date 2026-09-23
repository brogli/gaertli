import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const {
  mockStageFile,
  mockUnstageFile,
  mockStageAll,
  mockUnstageAll,
  mockDiscardFile,
  mockStatus,
  mockOpenFile,
  mockCommit,
  mockSetMessageDraft,
  mockRequire,
} = vi.hoisted(() => ({
  mockStageFile: vi.fn(),
  mockUnstageFile: vi.fn(),
  mockStageAll: vi.fn(),
  mockUnstageAll: vi.fn(),
  mockDiscardFile: vi.fn(),
  mockStatus: vi.fn(),
  mockOpenFile: vi.fn(),
  mockCommit: vi.fn(),
  mockSetMessageDraft: vi.fn(),
  mockRequire: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: {
    StageFile: mockStageFile,
    UnstageFile: mockUnstageFile,
    StageAll: mockStageAll,
    UnstageAll: mockUnstageAll,
    DiscardFile: mockDiscardFile,
    Status: mockStatus,
    OpenFile: mockOpenFile,
    Commit: mockCommit,
    SetMessageDraft: mockSetMessageDraft,
  },
}))

vi.mock('openvue/useconfirm', () => ({
  useConfirm: () => ({ require: mockRequire, close: vi.fn() }),
}))

import ChangesPanel from './ChangesPanel.vue'
import CommitBox from './CommitBox.vue'
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
    mockDiscardFile.mockReset().mockResolvedValue(undefined)
    mockStatus.mockReset().mockResolvedValue(empty)
    mockOpenFile.mockReset().mockResolvedValue(undefined)
    mockCommit.mockReset().mockResolvedValue(undefined)
    mockSetMessageDraft.mockReset().mockResolvedValue(undefined)
    mockRequire.mockReset()
  })

  it('FR-CHG-05 CommitBox renders between the header and the Staged group', () => {
    const wrapper = mount(ChangesPanel, { props })

    expect(wrapper.findComponent(CommitBox).exists()).toBe(true)
    const html = wrapper.html()
    expect(html.indexOf('</header>')).toBeLessThan(html.indexOf('class="commit-box"'))
    expect(html.indexOf('class="commit-box"')).toBeLessThan(html.indexOf('Staged'))
  })

  it('FR-CHG-08 switching worktree remounts CommitBox, so worktree A\'s typed text never carries into worktree B\'s draft', async () => {
    const wrapper = mount(ChangesPanel, { props })
    await wrapper.get('textarea').setValue('worktree A text')

    await wrapper.setProps({ wt: '/other' })

    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('')
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

    expect(changes.selectedFile).toEqual({ path: 'a.txt', oldPath: '', staged: false, status: 'M' })
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

  it('FR-CONF-02 double-clicking a conflicted row calls Facade.OpenFile with its path', async () => {
    const changes = useChangesStore()
    changes.files = { ...empty, conflicted: [{ path: 'c.txt', status: 'UU' }] }

    const wrapper = mount(ChangesPanel, { props })
    await wrapper.get('li').trigger('dblclick')

    expect(mockOpenFile).toHaveBeenCalledWith('/repo', 'c.txt', 0)
  })

  it('FR-CONF-03 clicking a conflicted row\'s "Mark resolved" button calls Facade.StageFile with its path', async () => {
    const changes = useChangesStore()
    changes.files = { ...empty, conflicted: [{ path: 'c.txt', status: 'UU' }] }

    const wrapper = mount(ChangesPanel, { props })
    expect(wrapper.get('[data-test="action"]').attributes('aria-label')).toBe('Mark resolved')
    await wrapper.get('[data-test="action"]').trigger('click')

    expect(mockStageFile).toHaveBeenCalledWith('/repo', 'c.txt')
  })

  it('FR-CONF-02, FR-CONF-03 a conflicted row is not selectable (no working diff to show)', async () => {
    const changes = useChangesStore()
    changes.files = { ...empty, conflicted: [{ path: 'c.txt', status: 'UU' }] }

    const wrapper = mount(ChangesPanel, { props })
    await wrapper.get('li').trigger('click')

    expect(changes.selectedFile).toBeNull()
  })

  it('FR-CHG-10, FR-UX-03, QA-SAFE-01 an unstaged row\'s "Discard" confirms naming the path, then calls Facade.DiscardFile untracked=false', async () => {
    const changes = useChangesStore()
    changes.files = { ...empty, unstaged: [{ path: 'a.txt', status: 'M' }] }

    const wrapper = mount(ChangesPanel, { props })
    const discard = wrapper
      .findAll('[data-test="second-action"]')
      .find((b) => b.attributes('aria-label') === 'Discard')
    await discard?.trigger('click')

    expect(mockRequire).toHaveBeenCalledTimes(1)
    const options = mockRequire.mock.calls[0]![0] as { message: string; accept: () => void }
    expect(options.message).toContain('a.txt')
    expect(mockDiscardFile).not.toHaveBeenCalled()

    options.accept()

    expect(mockDiscardFile).toHaveBeenCalledWith('/repo', 'a.txt', false)
  })

  it('FR-CHG-10 declining the confirm does not call Facade.DiscardFile', async () => {
    const changes = useChangesStore()
    changes.files = { ...empty, unstaged: [{ path: 'a.txt', status: 'M' }] }

    const wrapper = mount(ChangesPanel, { props })
    const discard = wrapper
      .findAll('[data-test="second-action"]')
      .find((b) => b.attributes('aria-label') === 'Discard')
    await discard?.trigger('click')

    expect(mockDiscardFile).not.toHaveBeenCalled()
  })

  it('FR-CHG-10 an untracked row\'s "Discard" confirm wording says the file will be deleted, then calls Facade.DiscardFile untracked=true', async () => {
    const changes = useChangesStore()
    changes.files = { ...empty, untracked: [{ path: 'new.txt', status: '?' }] }

    const wrapper = mount(ChangesPanel, { props })
    const discard = wrapper
      .findAll('[data-test="second-action"]')
      .find((b) => b.attributes('aria-label') === 'Discard')
    await discard?.trigger('click')

    const options = mockRequire.mock.calls[0]![0] as { message: string; accept: () => void }
    expect(options.message.toLowerCase()).toContain('delete')

    options.accept()

    expect(mockDiscardFile).toHaveBeenCalledWith('/repo', 'new.txt', true)
  })

  it('FR-CHG-10 the Staged group offers no Discard button', () => {
    const changes = useChangesStore()
    changes.files = { ...empty, staged: [{ path: 'a.txt', status: 'M' }] }

    const wrapper = mount(ChangesPanel, { props })

    expect(wrapper.findAll('[data-test="second-action"]').length).toBe(0)
  })
})
