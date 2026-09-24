import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade, SettingsService } = vi.hoisted(() => ({
  Facade: {
    CommitDetails: vi.fn(),
    CompareCommits: vi.fn(),
    CompareWithWorking: vi.fn(),
    Status: vi.fn(),
    DiffFile: vi.fn(),
    DiffRange: vi.fn(),
    DiffAgainstWorking: vi.fn(),
    DiffWorking: vi.fn(),
    StageLines: vi.fn(),
    OpenFile: vi.fn(),
  },
  SettingsService: { Get: vi.fn(), Set: vi.fn() },
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade,
  SettingsService,
}))

vi.mock('@wailsio/runtime', () => import('../testing/runtime'))
import { Events, Window, emit } from '../testing/runtime'

import DiffWindowView from './DiffWindowView.vue'
import { DiffKind, type DiffTarget } from '../lib/diffTarget'
import { defaultSettingsFixture } from '../testing/fixtures/settings'
import { diffFileFixture, hunk } from '../testing/fixtures/diff'
import { changedFilesFixture } from '../testing/fixtures/changes'

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

async function mountWindow(target: DiffTarget, wt = '/repo'): Promise<VueWrapper> {
  const wrapper = mount(DiffWindowView, { props: { wt, target } })
  await flushPromises()
  return wrapper
}

const rows = (wrapper: VueWrapper) => wrapper.findAll('.files li')
const selectedRow = (wrapper: VueWrapper) => wrapper.get('.files li.selected .path').text()

describe('DiffWindowView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    for (const fn of Object.values(Facade)) fn.mockReset()
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
    Facade.StageLines.mockResolvedValue(undefined)
    Facade.OpenFile.mockResolvedValue(undefined)
    SettingsService.Get.mockResolvedValue(defaultSettingsFixture())
    Events.On.mockClear()
    Window.Close.mockReset()
  })

  it("FR-DIFF-07 a commit's window lists the commit's files and shows the named one", async () => {
    const wrapper = await mountWindow({
      kind: DiffKind.CommitDiff,
      sha: 'abc',
      path: 'b.txt',
      oldPath: '',
    })

    expect(Facade.CommitDetails).toHaveBeenCalledWith('/repo', 'abc')
    expect(rows(wrapper)).toHaveLength(3)
    expect(selectedRow(wrapper)).toBe('b.txt')
    expect(Facade.DiffFile).toHaveBeenCalledWith('/repo', 'abc', 'b.txt', '', opts)
  })

  it("FR-DIFF-07 a comparison's window lists the compared files", async () => {
    const wrapper = await mountWindow({
      kind: DiffKind.RangeDiff,
      from: 'x',
      to: 'y',
      path: 'a.txt',
      oldPath: '',
    })

    expect(Facade.CompareCommits).toHaveBeenCalledWith('/repo', 'x', 'y')
    expect(rows(wrapper)).toHaveLength(3)
    expect(Facade.DiffRange).toHaveBeenCalledWith('/repo', 'x', 'y', 'a.txt', '', opts)
  })

  it("FR-DIFF-07 a working-tree comparison's window lists the files that differ from the working tree", async () => {
    const wrapper = await mountWindow({
      kind: DiffKind.AgainstWorkingDiff,
      sha: 'abc',
      path: 'c.txt',
      oldPath: '',
    })

    expect(Facade.CompareWithWorking).toHaveBeenCalledWith('/repo', 'abc')
    expect(rows(wrapper)).toHaveLength(3)
    expect(Facade.DiffAgainstWorking).toHaveBeenCalledWith('/repo', 'abc', 'c.txt', '', opts)
  })

  it("FR-DIFF-07 the working tree's window lists the changes in their groups; a conflicted file opens in the editor", async () => {
    const wrapper = await mountWindow({
      kind: DiffKind.WorkingDiff,
      staged: false,
      path: 'b.txt',
      oldPath: '',
    })

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
    expect(Facade.DiffWorking).toHaveBeenCalledWith('/repo', 'b.txt', '', false, opts)

    await rows(wrapper)[3]!.trigger('dblclick')
    expect(Facade.OpenFile).toHaveBeenCalledWith('/repo', 'both.txt', 0)
  })

  it('FR-DIFF-07 clicking a file in the list shows its diff', async () => {
    const wrapper = await mountWindow({
      kind: DiffKind.CommitDiff,
      sha: 'abc',
      path: 'a.txt',
      oldPath: '',
    })

    await rows(wrapper)[2]!.trigger('click')
    await flushPromises()

    expect(selectedRow(wrapper)).toBe('c.txt')
    expect(Facade.DiffFile).toHaveBeenLastCalledWith('/repo', 'abc', 'c.txt', '', opts)
  })

  it('FR-DIFF-02 next/previous file step through the list and stop at its ends', async () => {
    const wrapper = await mountWindow({
      kind: DiffKind.RangeDiff,
      from: 'x',
      to: 'y',
      path: 'b.txt',
      oldPath: '',
    })

    await wrapper.get('[data-test="next-file"]').trigger('click')
    await flushPromises()
    expect(selectedRow(wrapper)).toBe('c.txt')
    expect(Facade.DiffRange).toHaveBeenLastCalledWith('/repo', 'x', 'y', 'c.txt', '', opts)

    await wrapper.get('[data-test="next-file"]').trigger('click')
    await flushPromises()
    expect(selectedRow(wrapper)).toBe('c.txt')

    await wrapper.get('[data-test="prev-file"]').trigger('click')
    await wrapper.get('[data-test="prev-file"]').trigger('click')
    await flushPromises()
    expect(selectedRow(wrapper)).toBe('a.txt')
  })

  it("FR-DIFF-02 in the working tree's window, next file goes on from the staged group to the unstaged one", async () => {
    const wrapper = await mountWindow({
      kind: DiffKind.WorkingDiff,
      staged: true,
      path: 'a.txt',
      oldPath: '',
    })

    await wrapper.get('[data-test="next-file"]').trigger('click')
    await flushPromises()

    expect(Facade.DiffWorking).toHaveBeenLastCalledWith('/repo', 'b.txt', '', false, opts)
  })

  it('FR-DIFF-07 diff:show retargets the window, to another worktree too', async () => {
    const wrapper = await mountWindow({
      kind: DiffKind.CommitDiff,
      sha: 'abc',
      path: 'a.txt',
      oldPath: '',
    })

    emit('diff:show', {
      data: {
        wt: '/other',
        target: { kind: DiffKind.RangeDiff, from: 'x', to: 'y', path: 'b.txt' },
      },
    })
    await flushPromises()

    expect(Facade.CompareCommits).toHaveBeenCalledWith('/other', 'x', 'y')
    expect(Facade.DiffRange).toHaveBeenCalledWith('/other', 'x', 'y', 'b.txt', '', opts)
    expect(selectedRow(wrapper)).toBe('b.txt')
  })

  it("FR-DIFF-07 a diff:show for another main window's diff window is ignored", async () => {
    await mountWindow({ kind: DiffKind.CommitDiff, sha: 'abc', path: 'a.txt', oldPath: '' })

    emit('diff:show', {
      sender: 'window-2',
      data: { wt: '/repo', target: { kind: DiffKind.CommitDiff, sha: 'abc', path: 'b.txt' } },
    })
    await flushPromises()

    expect(Facade.DiffFile).not.toHaveBeenCalledWith('/repo', 'abc', 'b.txt', '', opts)
  })

  it('FR-CHG-03 staging a hunk from the window stages it; after repo:changed the list and the diff reload', async () => {
    const wrapper = await mountWindow({
      kind: DiffKind.WorkingDiff,
      staged: false,
      path: 'b.txt',
      oldPath: '',
    })

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
    emit('repo:changed', { data: { scope: 'index' } })
    await flushPromises()

    expect(Facade.Status.mock.calls.length).toBeGreaterThan(statusCalls)
    expect(Facade.DiffWorking.mock.calls.length).toBeGreaterThan(diffCalls)
  })

  it('FR-DIFF-07 Escape closes the window, unless it closes an open dialog first', async () => {
    await mountWindow({ kind: DiffKind.CommitDiff, sha: 'abc', path: 'a.txt', oldPath: '' })
    const dialog = document.createElement('div')
    dialog.setAttribute('role', 'alertdialog')
    document.body.append(dialog)

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    expect(Window.Close).not.toHaveBeenCalled()

    dialog.remove()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    expect(Window.Close).toHaveBeenCalledTimes(1)
  })

  it('FR-ED-02 a file of the list opens in the editor', async () => {
    const wrapper = await mountWindow({
      kind: DiffKind.CommitDiff,
      sha: 'abc',
      path: 'a.txt',
      oldPath: '',
    })

    await rows(wrapper)[1]!.get('[data-test="open-editor"]').trigger('click')

    expect(Facade.OpenFile).toHaveBeenCalledWith('/repo', 'b.txt', 0)
  })
})
