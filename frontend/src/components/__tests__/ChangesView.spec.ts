import { flushPromises, mount } from '@vue/test-utils'
import type { DOMWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mountNotices } from '@/testing/mountNotices'
import { editorNotFound } from '@/testing/errors'

vi.mock(
  '../../../bindings/github.com/brogli/nebegit/internal/app',
  () => import('@/testing/mainWindowApp'),
)
import { AppService, Facade, resetMainWindowApp } from '@/testing/mainWindowApp'

vi.mock('openvue/useconfirm', () => import('@/testing/confirm'))
import { confirmOptions, confirmRequire } from '@/testing/confirm'

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))

import ChangesView from '../ChangesView.vue'
import ChangedFiles from '../ChangedFiles.vue'
import CommitBox from '../CommitBox.vue'
import { useChangesStore } from '@/stores/changes'
import { changedFilesFixture } from '@/testing/fixtures/changes'
import { findButton } from '@/testing/findButton'
import type { ChangedFiles as ChangedFilesData } from '../../../bindings/github.com/brogli/nebegit/internal/core'

const props = { wt: '/repo' }

// mountWith mounts ChangesView over a change list holding files.
function mountWith(files: Partial<ChangedFilesData>) {
  useChangesStore().files = changedFilesFixture(files)
  return mount(ChangesView, { props })
}

// treeLabels reads a mounted Tree's items, in DOM order (folders first).
const treeLabels = (tree: DOMWrapper<Element>) =>
  tree.findAll('[role="treeitem"]').map((n) => n.attributes('aria-label'))

describe('ChangesView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    resetMainWindowApp()
    confirmRequire.mockReset()
    localStorage.clear()
  })

  it('FR-CHG-11 CommitBox renders after the change list, outside its scrolling region', () => {
    const wrapper = mount(ChangesView, { props })

    expect(wrapper.findComponent(CommitBox).exists()).toBe(true)
    const list = wrapper.get('.changes-body').element
    const box = wrapper.get('.commit-box').element
    expect(list.compareDocumentPosition(box) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(list.contains(box)).toBe(false)
  })

  it('FR-CHG-11 the commit box takes the fixed, bordered-off place ChangesView gives it', () => {
    const wrapper = mount(ChangesView, { props })

    expect(wrapper.get('.commit-box').classes()).toEqual(
      expect.arrayContaining(['flex-none', 'border-t']),
    )
  })

  it("FR-CHG-12 the commit message field has 6 rows and fills the commit box's width", () => {
    const wrapper = mount(ChangesView, { props })

    const textarea = wrapper.get('textarea')
    expect(textarea.attributes('rows')).toBe('6')
    expect(textarea.classes()).toContain('p-textarea-fluid')
  })

  it("FR-CHG-08 switching worktree remounts CommitBox, so worktree A's typed text never carries into worktree B's draft", async () => {
    const wrapper = mount(ChangesView, { props })
    await wrapper.get('textarea').setValue('worktree A text')

    await wrapper.setProps({ wt: '/other' })

    expect(wrapper.get('textarea').element.value).toBe('')
  })

  it("FR-CHG-02 clicking an unstaged row's Stage button calls Facade.StageFile with its path", async () => {
    const wrapper = mountWith({ unstaged: [{ path: 'a.txt', status: 'M' }] })
    await wrapper.get('[data-test="action"]').trigger('click')

    expect(Facade.StageFile).toHaveBeenCalledTimes(1)
    expect(Facade.StageFile).toHaveBeenCalledWith('/repo', 'a.txt')
  })

  it("FR-CHG-02 clicking a staged row's Unstage button calls Facade.UnstageFile with its path", async () => {
    const wrapper = mountWith({ staged: [{ path: 'a.txt', status: 'M' }] })
    await wrapper.get('[data-test="action"]').trigger('click')

    expect(Facade.UnstageFile).toHaveBeenCalledTimes(1)
    expect(Facade.UnstageFile).toHaveBeenCalledWith('/repo', 'a.txt')
  })

  it('FR-CHG-04 a click only marks a row; a double-click or Enter opens its side of the change in the diff window', async () => {
    const wrapper = mountWith({
      staged: [{ path: 'a.txt', status: 'M' }],
      unstaged: [{ path: 'a.txt', status: 'M' }],
    })
    const [staged, unstaged] = wrapper.findAll('li')

    await unstaged!.trigger('click')
    expect(unstaged!.classes()).toContain('selected')
    expect(staged!.classes()).not.toContain('selected')
    expect(AppService.OpenDiff).not.toHaveBeenCalled()

    await unstaged!.trigger('dblclick')
    await staged!.trigger('keydown', { key: 'Enter' })

    expect(AppService.OpenDiff.mock.calls).toEqual([
      ['/repo', { kind: 'working', staged: false, path: 'a.txt', oldPath: '' }],
      ['/repo', { kind: 'working', staged: true, path: 'a.txt', oldPath: '' }],
    ])
  })

  it('FR-ED-02 clicking a row\'s "Open in editor" control calls Facade.OpenFile, not select', async () => {
    const wrapper = mountWith({ unstaged: [{ path: 'a.txt', status: 'M' }] })
    await wrapper.get('[data-test="open-editor"]').trigger('click')

    expect(Facade.OpenFile).toHaveBeenCalledWith('/repo', 'a.txt', 0)
    expect(wrapper.get('li').classes()).not.toContain('selected')
  })

  it('FR-ED-06 OpenFile failing EDITOR_NOT_FOUND shows an error notice naming the command, no banner', async () => {
    Facade.OpenFile.mockRejectedValueOnce(editorNotFound('myed'))
    const wrapper = mountWith({ unstaged: [{ path: 'a.txt', status: 'M' }] })
    const notice = await mountNotices()

    await wrapper.get('[data-test="open-editor"]').trigger('click')
    await flushPromises()

    expect(notice.text()).toContain('myed')
    expect(wrapper.find('.error-banner').exists()).toBe(false)
  })

  it("FR-UX-15 a second EDITOR_NOT_FOUND failure replaces the first one's notice", async () => {
    Facade.OpenFile.mockRejectedValueOnce(editorNotFound('alpha')).mockRejectedValueOnce(
      editorNotFound('beta'),
    )
    const wrapper = mountWith({ unstaged: [{ path: 'a.txt', status: 'M' }] })
    const notice = await mountNotices()

    await wrapper.get('[data-test="open-editor"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="open-editor"]').trigger('click')
    await flushPromises()

    expect(notice.findAll('.p-toast-message')).toHaveLength(1)
    expect(notice.text()).toContain('beta')
  })

  it('FR-CHG-02 "Stage all" calls Facade.StageAll once; the index event reloads the list', async () => {
    const wrapper = mountWith({ unstaged: [{ path: 'a.txt', status: 'M' }] })
    const stageAll = findButton(wrapper, 'Stage all')
    await stageAll?.trigger('click')
    await flushPromises()

    expect(Facade.StageAll).toHaveBeenCalledWith('/repo')
    expect(Facade.Status).not.toHaveBeenCalled()
  })

  it('FR-CONF-02 double-clicking a conflicted row opens the diff window on it as a conflict', async () => {
    const wrapper = mountWith({ conflicted: [{ path: 'c.txt', status: 'UU' }] })
    await wrapper.get('li').trigger('dblclick')

    expect(AppService.OpenDiff).toHaveBeenCalledWith('/repo', {
      kind: 'working',
      conflicted: true,
      path: 'c.txt',
      oldPath: '',
    })
    expect(Facade.OpenFile).not.toHaveBeenCalled()
  })

  it('FR-CONF-02 a conflicted row\'s "Open in editor" button opens the editor', async () => {
    const wrapper = mountWith({ conflicted: [{ path: 'c.txt', status: 'UU' }] })
    await wrapper.get('[data-test="open-editor"]').trigger('click')

    expect(Facade.OpenFile).toHaveBeenCalledWith('/repo', 'c.txt', 0)
    expect(AppService.OpenDiff).not.toHaveBeenCalled()
  })

  it('FR-CONF-08 only a staged row of a resolved file has "Reopen conflict"; it confirms, then reopens', async () => {
    const wrapper = mountWith({
      staged: [
        { path: 'r.txt', status: 'M' },
        { path: 's.txt', status: 'M' },
      ],
      resolved: ['r.txt'],
    })
    const rows = wrapper.findAll('li')
    const reopen = (i: number) => rows[i]!.find('[data-test="second-action"]')

    expect(reopen(0).attributes('aria-label')).toBe('Reopen conflict')
    expect(reopen(1).exists()).toBe(false)

    await reopen(0).trigger('click')
    const options = confirmOptions()
    expect(options.header).toBe('Reopen conflict')
    expect(options.message).toBe('Reopen the conflict in r.txt? Its resolution is lost.')
    expect(Facade.ReopenConflict).not.toHaveBeenCalled()

    options.accept()
    expect(Facade.ReopenConflict).toHaveBeenCalledWith('/repo', 'r.txt')
  })

  it('FR-CONF-08 a resolved file without changes is listed under Conflicted and can be reopened', async () => {
    const wrapper = mountWith({ resolved: ['f.txt'] })
    const sections = wrapper.findAll('section section')
    const conflicted = sections[sections.length - 1]!
    const rows = conflicted.findAll('li')

    expect(rows).toHaveLength(1)
    expect(rows[0]!.text()).toContain('f.txt')
    expect(rows[0]!.find('[data-test="action"]').exists()).toBe(false)
    await rows[0]!.get('[data-test="second-action"]').trigger('click')
    const options = confirmOptions()
    expect(options.message).toBe('Reopen the conflict in f.txt? Its resolution is lost.')
    options.accept()
    expect(Facade.ReopenConflict).toHaveBeenCalledWith('/repo', 'f.txt')
  })

  it('FR-CONF-08 a resolved file with a staged change is listed only under Staged', () => {
    const wrapper = mountWith({ staged: [{ path: 'f.txt', status: 'M' }], resolved: ['f.txt'] })

    expect(wrapper.findAll('li')).toHaveLength(1)
  })

  it('FR-CONF-03 clicking a conflicted row\'s "Mark resolved" button calls Facade.StageFile with its path', async () => {
    const wrapper = mountWith({ conflicted: [{ path: 'c.txt', status: 'UU' }] })
    expect(wrapper.get('[data-test="action"]').attributes('aria-label')).toBe('Mark resolved')
    await wrapper.get('[data-test="action"]').trigger('click')

    expect(Facade.StageFile).toHaveBeenCalledWith('/repo', 'c.txt')
  })

  it('FR-CONF-02 a conflicted row is not selectable in the changes view', async () => {
    const wrapper = mountWith({ conflicted: [{ path: 'c.txt', status: 'UU' }] })
    await wrapper.get('li').trigger('click')

    expect(wrapper.get('li').classes()).not.toContain('selected')
  })

  it('FR-CHG-10, FR-UX-03, QA-SAFE-01 an unstaged row\'s "Discard" confirms naming the path, then calls Facade.DiscardFile untracked=false', async () => {
    const wrapper = mountWith({ unstaged: [{ path: 'a.txt', status: 'M' }] })
    await findButton(wrapper, 'Discard')!.trigger('click')

    expect(confirmRequire).toHaveBeenCalledTimes(1)
    const options = confirmOptions()
    expect(options.message).toContain('a.txt')
    expect(Facade.DiscardFile).not.toHaveBeenCalled()

    options.accept()

    expect(Facade.DiscardFile).toHaveBeenCalledWith('/repo', 'a.txt', false)
  })

  it('FR-CHG-10 declining the confirm does not call Facade.DiscardFile', async () => {
    const wrapper = mountWith({ unstaged: [{ path: 'a.txt', status: 'M' }] })
    await findButton(wrapper, 'Discard')!.trigger('click')

    expect(Facade.DiscardFile).not.toHaveBeenCalled()
  })

  it('FR-CHG-10 an untracked row\'s "Discard" confirm wording says the file will be deleted, then calls Facade.DiscardFile untracked=true', async () => {
    const wrapper = mountWith({ untracked: [{ path: 'new.txt', status: '?' }] })
    await findButton(wrapper, 'Discard')!.trigger('click')

    const options = confirmOptions()
    expect(options.message.toLowerCase()).toContain('delete')

    options.accept()

    expect(Facade.DiscardFile).toHaveBeenCalledWith('/repo', 'new.txt', true)
  })

  it('FR-UX-13 an untracked and a conflicted file show their status icons', () => {
    const wrapper = mountWith({
      untracked: [{ path: 'new.txt', status: '?' }],
      conflicted: [{ path: 'c.txt', status: 'UU' }],
    })

    const icons = wrapper.findAll('.status i').map((i) => i.classes())
    expect(icons).toContainEqual(['oi', 'oi-question-circle'])
    expect(icons).toContainEqual(['oi', 'oi-exclamation-triangle'])
  })

  it('FR-CHG-10 the Staged group offers no Discard button', () => {
    const wrapper = mountWith({ staged: [{ path: 'a.txt', status: 'M' }] })

    expect(wrapper.findAll('[data-test="second-action"]').length).toBe(0)
  })

  it('FR-CHG-13 the header toggle switches every group between flat lists and folder trees, folders first, each group showing only its own files', async () => {
    const wrapper = mountWith({
      staged: [{ path: 'sub/a.ts', status: 'M' }],
      unstaged: [{ path: 'b.txt', status: 'M' }],
      untracked: [{ path: 'n.txt', status: '?' }],
      conflicted: [{ path: 'c.txt', status: 'UU' }],
    })
    const header = wrapper.get('header')
    expect(header.text()).toContain('Stage all')
    expect(header.text()).toContain('Unstage all')
    const toggle = header.get('[data-test="file-tree-toggle"]')

    await toggle.trigger('click')

    expect(wrapper.findAll('h3').map((h) => h.text())).toEqual([
      'Staged',
      'Unstaged',
      'Untracked',
      'Conflicted',
    ])
    const trees = wrapper.findAll('[role="tree"]')
    expect(trees).toHaveLength(4)
    expect(treeLabels(trees[0]!)).toEqual(['sub', 'a.ts'])
    expect(treeLabels(trees[1]!)).toEqual(['b.txt'])
    expect(treeLabels(trees[2]!)).toEqual(['n.txt'])
    expect(treeLabels(trees[3]!)).toEqual(['c.txt'])

    await toggle.trigger('click')

    expect(wrapper.find('[role="tree"]').exists()).toBe(false)
    expect(wrapper.findAll('.files li')).toHaveLength(4)
  })

  it('FR-CHG-13 the changes view shares its choice with ChangedFiles, and a fresh mount starts from the remembered choice', async () => {
    const wrapper = mountWith({ staged: [{ path: 'a.txt', status: 'M' }] })
    const otherFiles = [{ path: 'x.txt', status: 'M' }]
    const other = mount(ChangedFiles, { props: { files: otherFiles, selected: null } })

    await wrapper.get('[data-test="file-tree-toggle"]').trigger('click')

    expect(other.find('[role="tree"]').exists()).toBe(true)
    expect(localStorage.getItem('nebegit.changedFiles.tree')).toBe('true')

    const fresh = mountWith({ staged: [{ path: 'a.txt', status: 'M' }] })
    expect(fresh.find('[role="tree"]').exists()).toBe(true)
  })

  it('FR-CHG-13 in tree mode, stage, unstage, discard and mark-resolved buttons on tree nodes call the right facade method with the file path', async () => {
    const wrapper = mountWith({
      staged: [{ path: 'a.txt', status: 'M' }],
      unstaged: [{ path: 'a.txt', status: 'M' }],
      untracked: [{ path: 'n.txt', status: '?' }],
      conflicted: [{ path: 'c.txt', status: 'UU' }],
    })
    await wrapper.get('[data-test="file-tree-toggle"]').trigger('click')
    const [staged, unstaged, untracked, conflicted] = wrapper.findAll('[role="tree"]')
    const button = (tree: DOMWrapper<Element>, label: string, test: string) =>
      tree.get(`[role="treeitem"][aria-label="${label}"] [data-test="${test}"]`)

    await button(staged!, 'a.txt', 'action').trigger('click')
    await button(unstaged!, 'a.txt', 'action').trigger('click')
    await button(untracked!, 'n.txt', 'second-action').trigger('click')
    confirmOptions().accept()
    await button(conflicted!, 'c.txt', 'action').trigger('click')

    expect(Facade.UnstageFile).toHaveBeenCalledWith('/repo', 'a.txt')
    expect(Facade.StageFile).toHaveBeenCalledWith('/repo', 'a.txt')
    expect(Facade.DiscardFile).toHaveBeenCalledWith('/repo', 'n.txt', true)
    expect(Facade.StageFile).toHaveBeenCalledWith('/repo', 'c.txt')
  })

  it('FR-CHG-13 in tree mode, a click selects a row per group and a double-click opens it the same way as list mode', async () => {
    const wrapper = mountWith({
      staged: [{ path: 'a.txt', status: 'M' }],
      unstaged: [{ path: 'a.txt', status: 'M' }],
      untracked: [{ path: 'n.txt', status: '?' }],
      conflicted: [{ path: 'c.txt', status: 'UU' }],
    })
    await wrapper.get('[data-test="file-tree-toggle"]').trigger('click')
    const [staged, unstaged, , conflicted] = wrapper.findAll('[role="tree"]')
    const row = (tree: DOMWrapper<Element>, label: string) =>
      tree.get(`[role="treeitem"][aria-label="${label}"] [data-pc-section="nodecontent"]`)

    await row(unstaged!, 'a.txt').trigger('click')

    expect(row(unstaged!, 'a.txt').attributes('data-p-selected')).toBe('true')
    expect(row(staged!, 'a.txt').attributes('data-p-selected')).toBe('false')

    await row(unstaged!, 'a.txt').trigger('dblclick')
    await row(conflicted!, 'c.txt').trigger('dblclick')
    await row(conflicted!, 'c.txt').trigger('click')

    expect(AppService.OpenDiff).toHaveBeenCalledWith('/repo', {
      kind: 'working',
      staged: false,
      path: 'a.txt',
      oldPath: '',
    })
    expect(AppService.OpenDiff).toHaveBeenCalledTimes(2)
    expect(AppService.OpenDiff).toHaveBeenLastCalledWith('/repo', {
      kind: 'working',
      conflicted: true,
      path: 'c.txt',
      oldPath: '',
    })
    expect(row(conflicted!, 'c.txt').attributes('data-p-selected')).toBe('false')
  })
})
