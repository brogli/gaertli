import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ChangeScope } from '../../../bindings/github.com/brogli/nebegit/internal/core'

vi.mock(
  '../../../bindings/github.com/brogli/nebegit/internal/app',
  () => import('@/testing/mainWindowApp'),
)
import { pressKey } from '@/testing/pressKey'
import { Facade, resetMainWindowApp } from '@/testing/mainWindowApp'

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))
vi.mock('openvue/useconfirm', () => import('@/testing/confirm'))
import { Events, Window, emit } from '@/testing/runtime'

import { confirmRequire } from '@/testing/confirm'
import { useMergeDrafts } from '@/composables/useMergeDrafts'
import DiffWindowView from '../DiffWindowView.vue'
import DiffView from '@/components/DiffView.vue'
import ConflictView from '@/components/ConflictView.vue'
import { DiffKind, type DiffTarget } from '@/lib/diffTarget'
import { diffFileFixture, hunk } from '@/testing/fixtures/diff'
import { changedFilesFixture } from '@/testing/fixtures/changes'

const opts = { force: false, ignoreWhitespace: false }
const files = [
  { path: 'a.txt', status: 'M' },
  { path: 'b.txt', status: 'M' },
  { path: 'c.txt', status: 'A' },
]

function diffOf(path: string) {
  return diffFileFixture({
    path,
    fingerprint: 'fp',
    oldContent: 'line1\nold\nline3\n',
    newContent: 'line1\nnew\nline3\n',
    hunks: [hunk(2, 1, 2, 1)],
  })
}

// mountWindow mounts the window with DiffView and ConflictView stubbed, for
// tests that look at the file list and the props the views get.
async function mountWindow(target: DiffTarget, wt = '/repo'): Promise<VueWrapper> {
  return mountWindowWith(target, wt, { DiffView: true, ConflictView: true })
}

// mountRealWindow mounts the window with the real DiffView and ConflictView.
async function mountRealWindow(target: DiffTarget): Promise<VueWrapper> {
  return mountWindowWith(target, '/repo', {})
}

async function mountWindowWith(
  target: DiffTarget,
  wt: string,
  stubs: Record<string, boolean>,
): Promise<VueWrapper> {
  const wrapper = mount(DiffWindowView, { props: { wt, target }, global: { stubs } })
  await flushPromises()
  return wrapper
}

const diffProps = (wrapper: VueWrapper) => wrapper.getComponent(DiffView).props()

// stepFile has the shown stubbed view ask for its neighbouring file.
async function stepFile(wrapper: VueWrapper, step: 1 | -1): Promise<void> {
  const view = wrapper.findComponent(DiffView)
  ;(view.exists() ? view : wrapper.getComponent(ConflictView)).vm.$emit('step-file', step)
  await flushPromises()
}

const commitA: DiffTarget = { kind: DiffKind.CommitDiff, sha: 'abc', path: 'a.txt', oldPath: '' }
const stagedA: DiffTarget = { kind: DiffKind.WorkingDiff, staged: true, path: 'a.txt', oldPath: '' }
const unstagedB: DiffTarget = {
  kind: DiffKind.WorkingDiff,
  staged: false,
  path: 'b.txt',
  oldPath: '',
}
const oneOfTwo: DiffTarget = {
  kind: DiffKind.WorkingDiff,
  conflicted: true,
  path: 'one.txt',
  oldPath: '',
}
const twoConflicted = () =>
  changedFilesFixture({
    conflicted: [
      { path: 'one.txt', status: 'UU' },
      { path: 'two.txt', status: 'UU' },
    ],
  })

const rows = (wrapper: VueWrapper) => wrapper.findAll('.files li')
const selectedRow = (wrapper: VueWrapper) => wrapper.get('.files li.selected .path').text()

describe('DiffWindowView', () => {
  beforeEach(() => {
    localStorage.clear()
    useMergeDrafts().keepOnly('/repo', [])
    setActivePinia(createPinia())
    resetMainWindowApp()
    Facade.CommitDetails.mockResolvedValue({ sha: 'abc', files })
    Facade.CompareCommits.mockResolvedValue(files)
    Facade.CompareWithWorking.mockResolvedValue(files)
    Facade.Status.mockResolvedValue(
      changedFilesFixture({
        staged: [{ path: 'a.txt', status: 'M' }],
        unstaged: [{ path: 'b.txt', status: 'M' }],
        untracked: [{ path: 'new.txt', status: '?' }],
        conflicted: [{ path: 'both.txt', status: 'U' }],
      }),
    )
    Facade.DiffFile.mockImplementation((_wt, _sha, path: string) => Promise.resolve(diffOf(path)))
    Facade.DiffAgainstWorking.mockImplementation((_wt, _sha, path: string) =>
      Promise.resolve(diffOf(path)),
    )
    Facade.DiffRange.mockImplementation((_wt, _a, _b, path: string) =>
      Promise.resolve(diffOf(path)),
    )
    Facade.DiffWorking.mockImplementation((_wt, path: string) => Promise.resolve(diffOf(path)))
    Facade.Conflict.mockResolvedValue({})
    Events.On.mockClear()
    Window.Close.mockReset()
  })

  it.each<[string, DiffTarget, () => void, string]>([
    [
      "FR-DIFF-07 a commit's window lists the commit's files and shows the named one",
      { kind: DiffKind.CommitDiff, sha: 'abc', path: 'b.txt', oldPath: '' },
      () => expect(Facade.CommitDetails).toHaveBeenCalledWith('/repo', 'abc'),
      'b.txt',
    ],
    [
      "FR-DIFF-07 a comparison's window lists the compared files",
      { kind: DiffKind.RangeDiff, from: 'x', to: 'y', path: 'a.txt', oldPath: '' },
      () => expect(Facade.CompareCommits).toHaveBeenCalledWith('/repo', 'x', 'y'),
      'a.txt',
    ],
    [
      "FR-DIFF-07 a working-tree comparison's window lists the files that differ from the working tree",
      { kind: DiffKind.AgainstWorkingDiff, sha: 'abc', path: 'c.txt', oldPath: '' },
      () => expect(Facade.CompareWithWorking).toHaveBeenCalledWith('/repo', 'abc'),
      'c.txt',
    ],
  ])('%s', async (_, target, expectListLoaded, path) => {
    const wrapper = await mountWindow(target)

    expectListLoaded()
    expect(rows(wrapper)).toHaveLength(3)
    expect(selectedRow(wrapper)).toBe(path)
    expect(diffProps(wrapper)).toMatchObject({ wt: '/repo', target })
  })

  it("FR-DIFF-07 the working tree's window lists the changes in their groups; a conflicted file shows its conflict", async () => {
    const wrapper = await mountWindow(unstagedB)

    expect(wrapper.findAll('h3').map((h) => h.text())).toEqual([
      'Staged',
      'Unstaged',
      'Untracked',
      'Conflicted',
    ])
    expect(rows(wrapper).map((r) => r.get('.path').text())).toEqual([
      'a.txt',
      'b.txt',
      'new.txt',
      'both.txt',
    ])
    expect(selectedRow(wrapper)).toBe('b.txt')
    expect(diffProps(wrapper)).toMatchObject({ target: { path: 'b.txt', staged: false } })
  })

  it('FR-CONF-02 selecting a conflicted file shows its conflict, not a diff', async () => {
    const wrapper = await mountWindow(unstagedB)

    await rows(wrapper)[3]!.trigger('click')
    await flushPromises()

    expect(selectedRow(wrapper)).toBe('both.txt')
    expect(wrapper.findComponent(DiffView).exists()).toBe(false)
    expect(wrapper.getComponent(ConflictView).props()).toEqual({ wt: '/repo', path: 'both.txt' })
  })

  it("FR-DIFF-02 FR-CONF-02 in the working tree's window, next file goes on from the staged group to the unstaged one, then to the conflicted files", async () => {
    const wrapper = await mountWindow(stagedA)

    await stepFile(wrapper, 1)
    expect(diffProps(wrapper)).toMatchObject({ target: { path: 'b.txt', staged: false } })

    await stepFile(wrapper, 1)
    await stepFile(wrapper, 1)
    expect(wrapper.getComponent(ConflictView).props('path')).toBe('both.txt')
  })

  it("FR-CONF-02 the conflict view's next and previous file step through the conflicted files", async () => {
    Facade.Status.mockResolvedValue(twoConflicted())
    const wrapper = await mountWindow(oneOfTwo)

    await stepFile(wrapper, 1)
    expect(wrapper.getComponent(ConflictView).props('path')).toBe('two.txt')

    await stepFile(wrapper, -1)
    expect(wrapper.getComponent(ConflictView).props('path')).toBe('one.txt')
  })

  describe('a conflicted target', () => {
    beforeEach(() => {
      Facade.Status.mockResolvedValue(twoConflicted())
    })

    async function indexChanges(status: ReturnType<typeof changedFilesFixture>): Promise<void> {
      Facade.Status.mockResolvedValue(status)
      emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
      await flushPromises()
    }

    it('FR-CONF-02 shows its conflict, then gives way to the next conflicted file once it leaves the list', async () => {
      const wrapper = await mountWindow(oneOfTwo)
      expect(wrapper.getComponent(ConflictView).props('path')).toBe('one.txt')
      await indexChanges(
        changedFilesFixture({
          staged: [{ path: 'one.txt', status: 'M' }],
          conflicted: [{ path: 'two.txt', status: 'UU' }],
        }),
      )
      expect(wrapper.getComponent(ConflictView).props('path')).toBe('two.txt')
      expect(selectedRow(wrapper)).toBe('two.txt')
    })

    it('FR-CONF-02 gives way to the conflicted file before it when the last one leaves the list', async () => {
      const wrapper = await mountWindow({ ...oneOfTwo, path: 'two.txt' })
      await indexChanges(
        changedFilesFixture({
          staged: [{ path: 'two.txt', status: 'M' }],
          conflicted: [{ path: 'one.txt', status: 'UU' }],
        }),
      )
      expect(wrapper.getComponent(ConflictView).props('path')).toBe('one.txt')
    })

    it('FR-CONF-02 shows a hint once no conflicted file is left', async () => {
      const wrapper = await mountWindow(oneOfTwo)
      await indexChanges(changedFilesFixture({ staged: [{ path: 'two.txt', status: 'M' }] }))
      expect(wrapper.findComponent(ConflictView).exists()).toBe(false)
      expect(wrapper.text()).toContain('No conflicts left.')
    })
  })

  it("FR-DIFF-07 the diff gets the window's target, with the chosen working-tree file's side and status; clicking a file in the list shows its diff", async () => {
    const wrapper = await mountRealWindow(stagedA)

    expect(diffProps(wrapper)).toMatchObject({
      wt: '/repo',
      target: { kind: DiffKind.WorkingDiff, path: 'a.txt', staged: true },
      status: 'M',
    })

    await rows(wrapper)[2]!.trigger('click')
    await flushPromises()

    expect(diffProps(wrapper)).toMatchObject({
      target: { kind: DiffKind.WorkingDiff, path: 'new.txt', staged: false },
      status: '?',
    })
    expect(selectedRow(wrapper)).toBe('new.txt')
    expect(Facade.DiffWorking).toHaveBeenLastCalledWith('/repo', 'new.txt', '', false, opts)
  })

  it('FR-DIFF-02 next/previous file step through the list and stop at its ends', async () => {
    const wrapper = await mountWindow({
      kind: DiffKind.RangeDiff,
      from: 'x',
      to: 'y',
      path: 'b.txt',
      oldPath: '',
    })

    await stepFile(wrapper, 1)
    expect(selectedRow(wrapper)).toBe('c.txt')
    expect(diffProps(wrapper)).toMatchObject({
      target: { kind: DiffKind.RangeDiff, path: 'c.txt' },
    })

    await stepFile(wrapper, 1)
    expect(selectedRow(wrapper)).toBe('c.txt')

    await stepFile(wrapper, -1)
    await stepFile(wrapper, -1)
    expect(selectedRow(wrapper)).toBe('a.txt')
  })

  it('FR-DIFF-07 diff:show retargets the window, to another worktree too', async () => {
    const wrapper = await mountWindow(commitA)

    emit('diff:show', {
      data: {
        wt: '/other',
        target: { kind: DiffKind.RangeDiff, from: 'x', to: 'y', path: 'b.txt' },
      },
    })
    await flushPromises()

    expect(Facade.CompareCommits).toHaveBeenCalledWith('/other', 'x', 'y')
    expect(diffProps(wrapper)).toMatchObject({
      wt: '/other',
      target: { kind: DiffKind.RangeDiff, from: 'x', to: 'y', path: 'b.txt' },
    })
    expect(selectedRow(wrapper)).toBe('b.txt')
  })

  it("FR-DIFF-07 a diff:show for another main window's diff window is ignored", async () => {
    const wrapper = await mountWindow(commitA)

    emit('diff:show', {
      sender: 'window-2',
      data: { wt: '/repo', target: { kind: DiffKind.CommitDiff, sha: 'abc', path: 'b.txt' } },
    })
    await flushPromises()

    expect(diffProps(wrapper)).toMatchObject({ target: { path: 'a.txt' } })
  })

  it('FR-CHG-03 staging a hunk from the window stages it; after repo:changed the list and the diff reload', async () => {
    const wrapper = await mountRealWindow(unstagedB)

    await wrapper.get('[data-test="stage-hunk"]').trigger('click')
    await flushPromises()
    expect(Facade.StageLines).toHaveBeenCalledWith(
      '/repo',
      'b.txt',
      { old: [2], new: [2], fingerprint: 'fp' },
      false,
    )

    const statusCalls = Facade.Status.mock.calls.length
    const diffCalls = Facade.DiffWorking.mock.calls.length
    emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
    await flushPromises()

    expect(Facade.Status.mock.calls.length).toBeGreaterThan(statusCalls)
    expect(Facade.DiffWorking.mock.calls.length).toBeGreaterThan(diffCalls)
  })

  it('FR-CONF-06 a draft of a file that is no longer conflicted is dropped', async () => {
    Facade.Status.mockResolvedValue(twoConflicted())
    await mountWindow(oneOfTwo)
    const drafts = useMergeDrafts()
    drafts.set('/repo', 'one.txt', '', () => ({ text: 'x', fingerprint: 'fp', regions: [] }))
    drafts.set('/repo', 'two.txt', '', () => ({ text: 'y', fingerprint: 'fp', regions: [] }))

    Facade.Status.mockResolvedValue(
      changedFilesFixture({ conflicted: [{ path: 'one.txt', status: 'UU' }] }),
    )
    emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
    await flushPromises()

    expect(drafts.get('/repo', 'one.txt', '')).toBeDefined()
    expect(drafts.get('/repo', 'two.txt', '')).toBeUndefined()
  })

  it('FR-DIFF-07 Escape closes the window, unless it closes an open dialog first', async () => {
    await mountWindow(commitA)
    const dialog = document.createElement('div')
    dialog.setAttribute('role', 'alertdialog')
    document.body.append(dialog)

    pressKey('Escape')
    expect(Window.Close).not.toHaveBeenCalled()

    dialog.remove()
    pressKey('Escape')
    await flushPromises()
    expect(Window.Close).toHaveBeenCalledTimes(1)
  })

  it('FR-CONF-06 Escape with an unsaved merge result closes the window without asking', async () => {
    confirmRequire.mockReset()
    await mountWindow(commitA)
    useMergeDrafts().set('/repo', 'a.txt', '', () => ({
      text: 'x',
      fingerprint: 'fp',
      regions: [],
    }))

    pressKey('Escape')
    await flushPromises()

    expect(confirmRequire).not.toHaveBeenCalled()
    expect(Window.Close).toHaveBeenCalledTimes(1)
  })

  it('FR-ED-02 a file of the list opens in the editor', async () => {
    const wrapper = await mountWindow(commitA)

    await rows(wrapper)[1]!.get('[data-test="open-editor"]').trigger('click')

    expect(Facade.OpenFile).toHaveBeenCalledWith('/repo', 'b.txt', 0)
  })

  it('FR-DIFF-10 the toggle switches the working tree groups to folder trees; a file there shows its diff', async () => {
    Facade.Status.mockResolvedValue(
      changedFilesFixture({
        staged: [{ path: 'src/a.txt', status: 'M' }],
        unstaged: [{ path: 'lib/b.txt', status: 'M' }],
      }),
    )
    const wrapper = await mountRealWindow({
      kind: DiffKind.WorkingDiff,
      staged: true,
      path: 'src/a.txt',
      oldPath: '',
    })

    await wrapper.get('[data-test="file-tree-toggle"]').trigger('click')

    expect(wrapper.findAll('[role="tree"]')).toHaveLength(2)
    expect(wrapper.find('.files').exists()).toBe(false)

    const node = wrapper.get('[role="treeitem"][aria-label="b.txt"]')
    await node.get('[data-pc-section="nodecontent"]').trigger('click')
    await flushPromises()

    expect(Facade.DiffWorking).toHaveBeenLastCalledWith('/repo', 'lib/b.txt', '', false, opts)
  })

  it('FR-DIFF-10 a choice already stored (by the commit detail view) shows the files as trees, the toggle pressed', async () => {
    localStorage.setItem('nebegit.changedFiles.tree', 'true')

    const wrapper = await mountWindow(commitA)

    expect(wrapper.find('[role="tree"]').exists()).toBe(true)
    expect(wrapper.find('.files').exists()).toBe(false)
    expect(wrapper.get('[data-test="file-tree-toggle"]').attributes('aria-pressed')).toBe('true')
  })

  it('ADR-4 after a WSL core restart the window reloads its file list and its diff', async () => {
    await mountRealWindow(commitA)

    emit('core:reconnected')
    await flushPromises()

    expect(Facade.CommitDetails).toHaveBeenCalledTimes(2)
    expect(Facade.DiffFile).toHaveBeenCalledTimes(2)
  })
})
