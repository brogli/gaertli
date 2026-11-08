// Requirements: FR-CHG-02, FR-CHG-04, FR-CHG-08, FR-CHG-10, FR-CHG-13, FR-CONF-02, FR-CONF-03, FR-CONF-08, FR-ED-02, FR-ED-06
import { flushPromises, mount, type DOMWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mountNotices } from '@/testing/mountNotices'
import { editorNotFound } from '@/testing/errors'

vi.mock(
  '../../../bindings/github.com/brogli/gaertli/internal/app',
  () => import('@/testing/mainWindowApp'),
)
import { AppService, Facade, resetMainWindowApp } from '@/testing/mainWindowApp'

vi.mock('openvue/useconfirm', () => import('@/testing/confirm'))
import { confirmOptions, confirmRequire } from '@/testing/confirm'

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))

import ChangesView from '../ChangesView.vue'
import ChangedFiles from '../ChangedFiles.vue'
import { useChangesStore } from '@/stores/changes'
import { changedFilesFixture } from '@/testing/fixtures/changes'
import { findButton } from '@/testing/findButton'
import type { ChangedFiles as ChangedFilesData } from '../../../bindings/github.com/brogli/gaertli/internal/core'

const props = { wt: '/repo' }

// mountWith mounts ChangesView over a change list holding files.
function mountWith(files: Partial<ChangedFilesData>) {
  useChangesStore().files = changedFilesFixture(files)
  return mount(ChangesView, { props })
}

type Mode = 'list' | 'tree'
const modes: Mode[] = ['list', 'tree']

// eachMode repeats every row once per file-list mode.
const eachMode = <T extends object>(rows: T[]) =>
  modes.flatMap((mode) => rows.map((row) => ({ mode, ...row })))

// mountIn mounts ChangesView over files, switched to folder trees for tree mode.
async function mountIn(mode: Mode, files: Partial<ChangedFilesData>) {
  const wrapper = mountWith(files)
  if (mode === 'tree') await wrapper.get('[data-test="file-tree-toggle"]').trigger('click')
  return wrapper
}

// rowIn is the group-th group's first row: an li, or a tree node's content.
function rowIn(wrapper: ReturnType<typeof mountWith>, mode: Mode, group: number) {
  return mode === 'list'
    ? wrapper.findAll('li')[group]!
    : wrapper.findAll('[role="tree"]')[group]!.get('[data-pc-section="nodecontent"]')
}

function isSelected(row: ReturnType<typeof rowIn>, mode: Mode): boolean {
  return mode === 'list'
    ? row.classes().includes('selected')
    : row.attributes('data-p-selected') === 'true'
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

  it("switching worktree remounts CommitBox, so worktree A's typed text never carries into worktree B's draft", async () => {
    const wrapper = mount(ChangesView, { props })
    await wrapper.get('textarea').setValue('worktree A text')

    await wrapper.setProps({ wt: '/other' })

    expect(wrapper.get('textarea').element.value).toBe('')
  })

  it.each(
    eachMode([
      {
        button: 'Stage',
        files: { unstaged: [{ path: 'a.txt', status: 'M' }] },
        call: Facade.StageFile,
      },
      {
        button: 'Unstage',
        files: { staged: [{ path: 'a.txt', status: 'M' }] },
        call: Facade.UnstageFile,
      },
    ]),
  )(
    "$mode: a row's $button button calls the facade with its path",
    async ({ mode, files, call }) => {
      const wrapper = await mountIn(mode, files)

      await wrapper.get('[data-test="action"]').trigger('click')

      expect(call).toHaveBeenCalledTimes(1)
      expect(call).toHaveBeenCalledWith('/repo', 'a.txt')
    },
  )

  it.each(modes)(
    '%s: a click only marks a row; a double-click or Enter opens its side of the change in the diff window',
    async (mode) => {
      const wrapper = await mountIn(mode, {
        staged: [{ path: 'a.txt', status: 'M' }],
        unstaged: [{ path: 'a.txt', status: 'M' }],
      })
      const staged = rowIn(wrapper, mode, 0)
      const unstaged = rowIn(wrapper, mode, 1)

      await unstaged.trigger('click')
      expect(isSelected(unstaged, mode)).toBe(true)
      expect(isSelected(staged, mode)).toBe(false)
      expect(AppService.OpenDiff).not.toHaveBeenCalled()

      await unstaged.trigger('dblclick')
      if (mode === 'list') await staged.trigger('keydown', { key: 'Enter' })
      else await staged.trigger('dblclick')

      expect(AppService.OpenDiff.mock.calls).toEqual([
        ['/repo', { kind: 'working', staged: false, path: 'a.txt', oldPath: '' }],
        ['/repo', { kind: 'working', staged: true, path: 'a.txt', oldPath: '' }],
      ])
    },
  )

  it('clicking a row\'s "Open in editor" control, also a conflicted one\'s, calls Facade.OpenFile, not select', async () => {
    const wrapper = mountWith({
      unstaged: [{ path: 'a.txt', status: 'M' }],
      conflicted: [{ path: 'c.txt', status: 'UU' }],
    })

    for (const button of wrapper.findAll('[data-test="open-editor"]')) await button.trigger('click')

    expect(Facade.OpenFile.mock.calls).toEqual([
      ['/repo', 'a.txt', 0],
      ['/repo', 'c.txt', 0],
    ])
    expect(AppService.OpenDiff).not.toHaveBeenCalled()
    expect(wrapper.findAll('li.selected')).toHaveLength(0)
  })

  it('OpenFile failing EDITOR_NOT_FOUND reaches the notice area, no banner', async () => {
    Facade.OpenFile.mockRejectedValueOnce(editorNotFound('myed'))
    const wrapper = mountWith({ unstaged: [{ path: 'a.txt', status: 'M' }] })
    const notice = await mountNotices()

    await wrapper.get('[data-test="open-editor"]').trigger('click')
    await flushPromises()

    expect(notice.findAll('.p-toast-message')).toHaveLength(1)
    expect(wrapper.find('.error-banner').exists()).toBe(false)
  })

  it('"Stage all" calls Facade.StageAll once and leaves reloading the list to the index event', async () => {
    const wrapper = mountWith({ unstaged: [{ path: 'a.txt', status: 'M' }] })
    await findButton(wrapper, 'Stage all')!.trigger('click')
    await flushPromises()

    expect(Facade.StageAll).toHaveBeenCalledWith('/repo')
    expect(Facade.Status).not.toHaveBeenCalled()
  })

  it.each(modes)(
    '%s: double-clicking a conflicted row opens the diff window on it as a conflict; a click does not select it',
    async (mode) => {
      const wrapper = await mountIn(mode, { conflicted: [{ path: 'c.txt', status: 'UU' }] })
      const row = rowIn(wrapper, mode, 0)

      await row.trigger('click')
      await row.trigger('dblclick')

      expect(isSelected(row, mode)).toBe(false)
      expect(AppService.OpenDiff).toHaveBeenCalledExactlyOnceWith('/repo', {
        kind: 'working',
        conflicted: true,
        path: 'c.txt',
        oldPath: '',
      })
      expect(Facade.OpenFile).not.toHaveBeenCalled()
    },
  )

  it('only a staged row of a resolved file has "Reopen conflict"; it confirms, then reopens', async () => {
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

  it('a resolved file without changes is listed under Conflicted and can be reopened', async () => {
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

  it('a resolved file with a staged change is listed only under Staged', () => {
    const wrapper = mountWith({ staged: [{ path: 'f.txt', status: 'M' }], resolved: ['f.txt'] })

    expect(wrapper.findAll('li')).toHaveLength(1)
  })

  it.each(modes)(
    '%s: a conflicted row\'s "Mark resolved" button calls Facade.StageFile with its path',
    async (mode) => {
      const wrapper = await mountIn(mode, { conflicted: [{ path: 'c.txt', status: 'UU' }] })
      expect(wrapper.get('[data-test="action"]').attributes('aria-label')).toBe('Mark resolved')

      await wrapper.get('[data-test="action"]').trigger('click')

      expect(Facade.StageFile).toHaveBeenCalledWith('/repo', 'c.txt')
    },
  )

  it.each(
    eachMode([
      {
        file: 'a.txt',
        files: { unstaged: [{ path: 'a.txt', status: 'M' }] },
        wording: /a\.txt/,
        untracked: false,
      },
      {
        file: 'new.txt',
        files: { untracked: [{ path: 'new.txt', status: '?' }] },
        wording: /delete/i,
        untracked: true,
      },
    ]),
  )(
    '$mode: "Discard" of $file confirms before calling Facade.DiscardFile untracked=$untracked',
    async ({ mode, file, files, wording, untracked }) => {
      const wrapper = await mountIn(mode, files)
      await wrapper.get('[data-test="second-action"]').trigger('click')

      expect(confirmRequire).toHaveBeenCalledTimes(1)
      const options = confirmOptions()
      expect(options.message).toMatch(wording)
      expect(Facade.DiscardFile).not.toHaveBeenCalled()

      options.accept()

      expect(Facade.DiscardFile).toHaveBeenCalledWith('/repo', file, untracked)
    },
  )

  it('the Staged group offers no Discard button', () => {
    const wrapper = mountWith({ staged: [{ path: 'a.txt', status: 'M' }] })

    expect(wrapper.findAll('[data-test="second-action"]').length).toBe(0)
  })

  it('the header toggle switches every group between flat lists and folder trees, folders first, each group showing only its own files', async () => {
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
  })

  it('the changes view shares its choice with ChangedFiles, and a fresh mount starts from the remembered choice', async () => {
    const wrapper = mountWith({ staged: [{ path: 'a.txt', status: 'M' }] })
    const other = mount(ChangedFiles, {
      props: { files: [{ path: 'x.txt', status: 'M' }], selected: null },
    })

    await wrapper.get('[data-test="file-tree-toggle"]').trigger('click')

    expect(other.find('[role="tree"]').exists()).toBe(true)
    expect(localStorage.getItem('gaertli.changedFiles.tree')).toBe('true')

    const fresh = mountWith({ staged: [{ path: 'a.txt', status: 'M' }] })
    expect(fresh.find('[role="tree"]').exists()).toBe(true)
  })
})
