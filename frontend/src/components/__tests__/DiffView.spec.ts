// Requirements: FR-CHG-03, FR-CHG-04, FR-CHG-10, FR-CHG-14, FR-CMP-02, FR-CMP-04, FR-DIFF-01, FR-DIFF-02, FR-DIFF-03, FR-DIFF-04, FR-DIFF-06, FR-DIFF-08, FR-DIFF-09, FR-DIFF-14, FR-DIFF-15, FR-DIFF-16, FR-DIFF-17, FR-DIFF-18, FR-DIFF-19, FR-DIFF-20, FR-DIFF-21, FR-DIFF-22, FR-DIFF-23, FR-DIFF-24, FR-ED-06, FR-ED-09, FR-REPO-04, FR-UX-09, FR-UX-16, QA-COMP-01
import { DOMWrapper, flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { EditorSelection } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import { getChunks } from '@codemirror/merge'

const { Facade, SettingsService } = vi.hoisted(() => ({
  Facade: {
    DiffFile: vi.fn(),
    DiffRange: vi.fn(),
    DiffWorking: vi.fn(),
    DiffAgainstWorking: vi.fn(),
    OpenFile: vi.fn(),
    StageLines: vi.fn(),
    DiscardLines: vi.fn(),
    Status: vi.fn(),
  },
  SettingsService: { Get: vi.fn(), Set: vi.fn() },
}))

vi.mock('../../../bindings/github.com/brogli/gaertli/internal/app', () => ({
  Facade,
  SettingsService,
}))

vi.mock('openvue/useconfirm', () => import('@/testing/confirm'))
import { mountNotices } from '@/testing/mountNotices'
import { editorNotFound, wailsError } from '@/testing/errors'
import { confirmOptions, confirmRequire } from '@/testing/confirm'

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))
import { Events, emit } from '@/testing/runtime'

import DiffView from '../DiffView.vue'
import { DiffKind } from '@/lib/diffTarget'
import { useSettingsStore } from '@/stores/settings'
import { defaultSettingsFixture } from '@/testing/fixtures/settings'
import { DiffMode } from '../../../bindings/github.com/brogli/gaertli/internal/settings'
import { changedFilesFixture } from '@/testing/fixtures/changes'
import { findButton } from '@/testing/findButton'
import { editorViews, lineNumbersOf, selectedLineOf } from '@/testing/editorView'
import { clickMenuItem } from '@/testing/contextMenu'
import { pressKey } from '@/testing/pressKey'
import { matchesOf, pressEnterIn, searchFieldOf, searchFor } from '@/testing/search'
import { diffFileFixture, hunk } from '@/testing/fixtures/diff'
import {
  ChangeScope,
  Code,
  type DiffFile,
} from '../../../bindings/github.com/brogli/gaertli/internal/core'

const settings = defaultSettingsFixture()

// editorOf is the mounted unified view.
function editorOf(wrapper: VueWrapper): EditorView {
  const [view] = editorViews(wrapper)
  if (!view) throw new Error('no EditorView mounted')
  return view
}

// chunkLines is the mounted unified view's chunks as the first and last new-side
// line each covers.
function chunkLines(wrapper: VueWrapper): [number, number][] {
  const { state } = editorOf(wrapper)
  return (getChunks(state)?.chunks ?? []).map((c) => [
    state.doc.lineAt(c.fromB).number,
    state.doc.lineAt(c.endB).number,
  ])
}

const base = { wt: '/repo' }

// oneLine changes line 1 from "old" to "new".
const oneLine = { oldContent: 'old\n', newContent: 'new\n', hunks: [hunk(1, 1, 1, 1)] }

// twoHunks changes lines 2 and 6.
const twoHunks = {
  oldContent: 'line1\nline2 old\nline3\nline4\nline5\nline6 old\nline7\n',
  newContent: 'line1\nline2 new\nline3\nline4\nline5\nline6 new\nline7\n',
  hunks: [hunk(2, 1, 2, 1), hunk(6, 1, 6, 1)],
}

// threeLines changes lines 2-4 from A, B, C to X, Y, Z.
const threeLines = {
  oldContent: 'line1\nA\nB\nC\nline5\n',
  newContent: 'line1\nX\nY\nZ\nline5\n',
  hunks: [hunk(2, 3, 2, 3)],
}

// selectText selects the new-side text from from's start to to's end.
function selectText(wrapper: VueWrapper, from: string, to: string): void {
  const view = editorOf(wrapper)
  const text = view.state.doc.toString()
  view.dispatch({
    selection: EditorSelection.range(text.indexOf(from), text.indexOf(to) + to.length),
  })
}

// mountCommit mounts commit abc123's diff of path and waits for it to load.
async function mountCommit(path = 'a.txt', oldPath = ''): Promise<VueWrapper> {
  const wrapper = mount(DiffView, {
    props: { ...base, target: { kind: DiffKind.CommitDiff, sha: 'abc123', path, oldPath } },
  })
  await flushPromises()
  return wrapper
}

// mountWorking mounts a.txt's working diff with fingerprint "fp", unstaged
// and modified unless working says otherwise.
async function mountWorking(
  file: Partial<DiffFile>,
  working: Partial<{ oldPath: string; staged: boolean; status: string }> = {},
) {
  Facade.DiffWorking.mockResolvedValue(diffFileFixture({ fingerprint: 'fp', ...file }))
  const wrapper = mount(DiffView, {
    props: {
      ...base,
      ...workingProps(working),
    },
  })
  await flushPromises()
  return wrapper
}

// workingProps are a.txt's working-tree diff, unstaged and modified unless
// working says otherwise.
function workingProps({
  status = 'M',
  ...target
}: Partial<{ oldPath: string; staged: boolean; status: string }> = {}) {
  return {
    target: { kind: DiffKind.WorkingDiff, path: 'a.txt', oldPath: '', staged: false, ...target },
    status,
  }
}

describe('DiffView', () => {
  // App.vue loads the settings store once for every view; DiffView reads
  // it rather than loading it itself, so its tests preload it too.
  beforeEach(async () => {
    setActivePinia(createPinia())
    localStorage.clear()
    for (const fn of Object.values(Facade)) fn.mockReset().mockResolvedValue(undefined)
    Facade.Status.mockResolvedValue(changedFilesFixture())
    SettingsService.Get.mockReset().mockResolvedValue(settings)
    SettingsService.Set.mockReset().mockResolvedValue(undefined)
    confirmRequire.mockReset()
    await useSettingsStore().load()
  })

  it('toggling "ignore whitespace" refetches through git and shows the hunks it returns', async () => {
    const file = diffFileFixture({
      oldContent: 'one\ntwo\nthree\n',
      newContent: 'one\ntwo  \nTHREE\n',
    })
    Facade.DiffFile.mockImplementation((_wt, _sha, _path, _oldPath, opts) =>
      Promise.resolve({
        ...file,
        hunks: opts.ignoreWhitespace ? [hunk(3, 1, 3, 1)] : [hunk(2, 2, 2, 2)],
      }),
    )

    const wrapper = await mountCommit()
    expect(chunkLines(wrapper)).toEqual([[2, 3]])
    expect(wrapper.get('[data-test="ignore-whitespace-toggle"]').text()).toBe('Showing whitespace')

    await wrapper.get('[data-test="ignore-whitespace-toggle"]').trigger('click')
    await flushPromises()

    expect(Facade.DiffFile).toHaveBeenLastCalledWith('/repo', 'abc123', 'a.txt', '', {
      force: false,
      ignoreWhitespace: true,
    })
    expect(chunkLines(wrapper)).toEqual([[3, 3]])
    expect(SettingsService.Set).toHaveBeenCalledWith(
      expect.objectContaining({ ignoreWhitespace: true }),
    )
    expect(wrapper.get('[data-test="ignore-whitespace-toggle"]').text()).toBe('Ignoring whitespace')
  })

  it("a line holding a lone \\r stays one line, so git's hunks sit on git's lines", async () => {
    const wrapper = await mountWorking({
      oldContent: 'one\ntwo\nth\rree\nfour\nfive\nsix\n',
      newContent: 'one\ntwo\nth\rree\nfour\nFIVE\nSIX\n',
      hunks: [hunk(5, 2, 5, 2)],
    })
    expect(wrapper.findAll('.cm-changedLine').map((l) => l.text())).toEqual(['FIVE', 'SIX'])

    selectText(wrapper, 'FIVE', 'SIX')
    await flushPromises()
    await wrapper.get('[data-test="stage-selected-lines"]').trigger('click')
    await flushPromises()

    expect(Facade.StageLines).toHaveBeenCalledWith(
      '/repo',
      'a.txt',
      { old: [5, 6], new: [5, 6], fingerprint: 'fp' },
      false,
    )
  })

  it('toggling "ignore whitespace" keeps the diff shown when git returns the same hunks', async () => {
    Facade.DiffFile.mockResolvedValue(
      diffFileFixture({
        oldContent: 'a\n',
        newContent: 'b\n',
        hunks: [hunk(1, 1, 1, 1)],
      }),
    )

    const wrapper = await mountCommit()
    await wrapper.get('[data-test="ignore-whitespace-toggle"]').trigger('click')
    await flushPromises()

    expect(Facade.DiffFile).toHaveBeenCalledTimes(2)
    expect(chunkLines(wrapper)).toEqual([[1, 1]])
  })

  it('a working-tree diff refetches on an outside change to the working tree', async () => {
    Events.On.mockClear()
    await mountWorking({ oldContent: 'a\n', newContent: 'b\n' })

    emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
    await flushPromises()

    expect(Facade.DiffWorking).toHaveBeenCalledTimes(2)
  })

  it('a selection survives an outside-change refetch, then the diff window handing back the same target', async () => {
    // Same shape as the diff window: an outside-change refetch (fetch
    // returns a new but equal DiffFile) followed by the window handing
    // DiffView a new but equal target (a diff:show for the file already
    // shown), which reruns the target watch and reads the diff store's cache.
    const wrapper = await mountWorking(threeLines)
    Facade.DiffWorking.mockImplementation(() =>
      Promise.resolve(diffFileFixture({ fingerprint: 'fp', ...threeLines })),
    )

    const view = editorOf(wrapper)
    // "line1\n" = 6 chars; "X\n"=2 (6-8); "Y\n"=2 (8-10): inside the chunk.
    view.dispatch({ selection: EditorSelection.range(8, 10) })
    await flushPromises()
    expect(wrapper.get('[data-test="stage-selected-lines"]').attributes('disabled')).toBeUndefined()

    emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
    await flushPromises()
    await wrapper.setProps(workingProps())
    await flushPromises()

    expect(wrapper.get('[data-test="stage-selected-lines"]').attributes('disabled')).toBeUndefined()
  })

  it("a commit's diff does not refetch on a working-tree change", async () => {
    Events.On.mockClear()
    Facade.DiffFile.mockResolvedValue(
      diffFileFixture({
        oldContent: 'a\n',
        newContent: 'b\n',
      }),
    )
    await mountCommit()

    emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
    await flushPromises()

    expect(Facade.DiffFile).toHaveBeenCalledTimes(1)
  })

  it('unified mode shows the computed diff; the mode toggle persists the choice and switches to side-by-side panes', async () => {
    Facade.DiffFile.mockResolvedValue(
      diffFileFixture({
        oldContent: 'shared line\nold only line\n',
        newContent: 'shared line\nnew only line\n',
        hunks: [hunk(2, 1, 2, 1)],
      }),
    )

    const wrapper = await mountCommit()
    expect(wrapper.findAll('.cm-editor')).toHaveLength(1)
    expect(wrapper.text()).toContain('old only line')
    expect(wrapper.text()).toContain('new only line')

    await wrapper.get('[data-test="mode-toggle"]').trigger('click')
    await flushPromises()

    expect(SettingsService.Set).toHaveBeenCalledTimes(1)
    expect(SettingsService.Set.mock.calls[0]?.[0]).toMatchObject({
      diffMode: DiffMode.SideBySideDiff,
    })
    expect(wrapper.findAll('.cm-editor')).toHaveLength(2)
  })

  it.each([
    ['binary', { binary: true }],
    ['too large', { tooLarge: true }],
  ])(
    'a %s file shows a placeholder without CodeMirror and keeps Previous/Next file, so stepping through files goes on past it',
    async (kind, flags) => {
      Facade.DiffFile.mockResolvedValue(diffFileFixture({ path: 'img.png', ...flags }))

      const wrapper = await mountCommit('img.png')

      expect(wrapper.text().toLowerCase()).toContain(kind)
      expect(wrapper.find('.cm-editor').exists()).toBe(false)
      expect(wrapper.find('[data-test="prev-file"]').exists()).toBe(true)
      expect(wrapper.find('[data-test="next-file"]').exists()).toBe(true)
      expect(wrapper.find('[data-test="next-change"]').exists()).toBe(false)
    },
  )

  it('shows a too-large placeholder, then loads the full diff on "Load anyway"', async () => {
    Facade.DiffFile.mockResolvedValueOnce(diffFileFixture({ path: 'big.txt', tooLarge: true }))

    const wrapper = await mountCommit()

    expect(wrapper.text().toLowerCase()).toContain('too large')
    expect(wrapper.find('.cm-editor').exists()).toBe(false)

    Facade.DiffFile.mockResolvedValueOnce(diffFileFixture({ path: 'big.txt', ...oneLine }))
    const loadAnyway = findButton(wrapper, 'Load anyway')
    await loadAnyway?.trigger('click')
    await flushPromises()

    expect(Facade.DiffFile).toHaveBeenLastCalledWith('/repo', 'abc123', 'a.txt', '', {
      force: true,
      ignoreWhitespace: false,
    })
    expect(wrapper.find('.cm-editor').exists()).toBe(true)
  })

  it('next/previous file ask for the neighbouring file', async () => {
    Facade.DiffFile.mockResolvedValue(diffFileFixture(oneLine))

    const wrapper = await mountCommit()
    await wrapper.get('[data-test="next-file"]').trigger('click')
    await wrapper.get('[data-test="prev-file"]').trigger('click')

    expect(wrapper.emitted('step-file')).toEqual([[1], [-1]])
  })

  it.each([
    [
      'a range target loads via DiffRange',
      {
        target: {
          kind: DiffKind.RangeDiff,
          from: 'sha-a',
          to: 'sha-b',
          path: 'a.txt',
          oldPath: '',
        },
      },
      'DiffRange',
      ['/repo', 'sha-a', 'sha-b', 'a.txt', ''],
    ],
    [
      'an against-working target loads via DiffAgainstWorking',
      { target: { kind: DiffKind.AgainstWorkingDiff, sha: 'sha-a', path: 'a.txt', oldPath: '' } },
      'DiffAgainstWorking',
      ['/repo', 'sha-a', 'a.txt', ''],
    ],
    [
      'a working-tree target loads via DiffWorking',
      workingProps({ staged: true }),
      'DiffWorking',
      ['/repo', 'a.txt', '', true],
    ],
  ] as const)('%s', async (_, targetProps, loader, args) => {
    Facade[loader].mockResolvedValue(diffFileFixture(oneLine))

    mount(DiffView, { props: { ...base, ...targetProps } })
    await flushPromises()

    expect(Facade[loader]).toHaveBeenCalledTimes(1)
    expect(Facade[loader]).toHaveBeenCalledWith(...args, { force: false, ignoreWhitespace: false })
    expect(Facade.DiffFile).not.toHaveBeenCalled()
  })

  it('"Open in editor" opens at the mounted view\'s current cursor line', async () => {
    Facade.DiffFile.mockResolvedValue(
      diffFileFixture({
        oldContent: 'line1\nline2\nline3\nline4\nline5\nline6\nline7\n',
        newContent: 'line1\nline2\nline3\nline4\nline5\nline6\nline7\n',
      }),
    )

    const wrapper = await mountCommit()

    const view = editorOf(wrapper)
    view.dispatch({ selection: EditorSelection.cursor(view.state.doc.line(5).from) })

    await wrapper.get('[data-test="open-editor"]').trigger('click')

    expect(Facade.OpenFile).toHaveBeenCalledWith('/repo', 'a.txt', 5)
  })

  it('in side-by-side mode, "Open in editor" opens at the new/right side\'s cursor line', async () => {
    Facade.DiffFile.mockResolvedValue(
      diffFileFixture({
        oldContent: 'line1\nline2\nline3\n',
        newContent: 'line1\nlineTWO\nlineTHREE\nline4\nline5\n',
        hunks: [hunk(2, 2, 2, 4)],
      }),
    )

    const wrapper = await mountCommit()
    await wrapper.get('[data-test="mode-toggle"]').trigger('click')
    await flushPromises()

    const views = editorViews(wrapper)
    expect(views).toHaveLength(2)
    const newView = views[1]!
    newView.dispatch({ selection: EditorSelection.cursor(newView.state.doc.line(4).from) })

    await wrapper.get('[data-test="open-editor"]').trigger('click')

    expect(Facade.OpenFile).toHaveBeenCalledWith('/repo', 'a.txt', 4)
  })

  it('a deleted-lines-only region has no line of its own: the cursor lands on a real new-side line', async () => {
    Facade.DiffFile.mockResolvedValue(
      diffFileFixture({
        oldContent: 'line1\nline2\nline3\nline4\nline5\n',
        newContent: 'line1\nline5\n', // line2-4 deleted entirely, no new-side counterpart
        hunks: [hunk(2, 3, 1, 0)],
      }),
    )

    const wrapper = await mountCommit()

    const view = editorOf(wrapper)
    // The line right after the deleted block: doc has no line for the
    // deletion itself to place a cursor on.
    view.dispatch({ selection: EditorSelection.cursor(view.state.doc.line(2).from) })

    await wrapper.get('[data-test="open-editor"]').trigger('click')

    expect(Facade.OpenFile).toHaveBeenCalledWith('/repo', 'a.txt', 2)
  })

  it('"Open in editor" with nothing mounted (too-large placeholder) opens at line 0', async () => {
    Facade.DiffFile.mockResolvedValue(diffFileFixture({ path: 'big.txt', tooLarge: true }))

    const wrapper = await mountCommit('big.txt')

    await wrapper.get('[data-test="open-editor"]').trigger('click')

    expect(Facade.OpenFile).toHaveBeenCalledWith('/repo', 'big.txt', 0)
  })

  it('the toolbar\'s "Open in editor" failing EDITOR_NOT_FOUND shows a notice, the diff stays', async () => {
    Facade.DiffFile.mockResolvedValue(diffFileFixture(oneLine))
    Facade.OpenFile.mockRejectedValueOnce(editorNotFound('myed'))
    const wrapper = await mountCommit()
    const notice = await mountNotices()

    await wrapper.get('[data-test="open-editor"]').trigger('click')
    await flushPromises()

    expect(notice.text()).toContain('myed')
    expect(wrapper.find('.cm-editor').exists()).toBe(true)
    expect(wrapper.find('.error-banner').exists()).toBe(false)
  })

  it('an outside change refetches the diff but keeps a LOCKED "Stage hunk"\'s Try again', async () => {
    const wrapper = await mountWorking({
      oldContent: 'a\nold\nb\n',
      newContent: 'a\nnew\nb\n',
      hunks: [hunk(2, 1, 2, 1)],
    })
    Facade.StageLines.mockRejectedValueOnce(wailsError('locked', Code.Locked))
    await wrapper.get('[data-test="stage-hunk"]').trigger('click')
    await flushPromises()
    expect(findButton(wrapper, 'Try again')).toBeTruthy()

    emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
    await flushPromises()

    expect(Facade.DiffWorking).toHaveBeenCalledTimes(2)
    expect(findButton(wrapper, 'Try again')).toBeTruthy()
  })

  it("an outside change clears a line action's error that offers no retry, showing the diff again", async () => {
    const wrapper = await mountWorking({
      oldContent: 'a\nold\nb\n',
      newContent: 'a\nnew\nb\n',
      hunks: [hunk(2, 1, 2, 1)],
    })
    Facade.StageLines.mockRejectedValueOnce(
      wailsError('a.txt changed since it was shown', Code.Internal),
    )
    await wrapper.get('[data-test="stage-hunk"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="stage-hunk"]').exists()).toBe(false)

    emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
    await flushPromises()

    expect(wrapper.text()).not.toContain('changed since it was shown')
    expect(wrapper.find('[data-test="stage-hunk"]').exists()).toBe(true)
  })

  it('with an unstaged working-tree target, clicking a hunk\'s "Stage hunk" button stages it and reloads', async () => {
    const wrapper = await mountWorking(twoHunks)

    const buttons = wrapper.findAll('[data-test="stage-hunk"]')
    expect(buttons.length).toBeGreaterThanOrEqual(2)
    expect(buttons[0]!.text()).toBe('Stage hunk')

    await buttons[0]!.trigger('click')
    await flushPromises()

    // The first hunk is the single-line substitution at line 2, the same
    // line number on both diff sides.
    expect(Facade.StageLines).toHaveBeenCalledTimes(1)
    expect(Facade.StageLines).toHaveBeenCalledWith(
      '/repo',
      'a.txt',
      { old: [2], new: [2], fingerprint: 'fp' },
      false,
    )
    expect(Facade.Status).toHaveBeenCalledTimes(1)
    expect(Facade.DiffWorking).toHaveBeenCalledTimes(2)
  })

  it.each([
    ['a blank line replaced', 'a\n\nb\n', 'a\nX\nb\n', [hunk(2, 1, 2, 1)], 0, [2], [2]],
    ['a line replaced by a blank one', 'a\nX\nb\n', 'a\n\nb\n', [hunk(2, 1, 2, 1)], 0, [2], [2]],
    ['a lone blank line deleted', 'a\n\nb\n', 'a\nb\n', [hunk(2, 1, 1, 0)], 0, [2], []],
    [
      'the second of two hunks',
      'l1\nl2\nl3\nl4\nl5\nl6\nl7\n',
      'l1\nl3\nl4\nl5\nL6\nL6b\nl7\n',
      [hunk(2, 1, 1, 0), hunk(6, 1, 5, 2)],
      1,
      [6],
      [5, 6],
    ],
  ])(
    '"Stage hunk" and "Discard hunk" on %s send exactly git\'s hunk',
    async (_, oldContent, newContent, hunks, nth, old, added) => {
      const wrapper = await mountWorking({ oldContent, newContent, hunks })
      const lines = { old, new: added, fingerprint: 'fp' }

      await wrapper.findAll('[data-test="stage-hunk"]')[nth]!.trigger('click')
      await flushPromises()
      expect(Facade.StageLines).toHaveBeenCalledWith('/repo', 'a.txt', lines, false)

      await wrapper.findAll('[data-test="discard-hunk"]')[nth]!.trigger('click')
      confirmOptions().accept()
      await flushPromises()
      expect(Facade.DiscardLines).toHaveBeenCalledWith('/repo', 'a.txt', lines)
    },
  )

  it('"Stage selected lines" across hunks joined around a blank line sends only the lines git changed', async () => {
    const wrapper = await mountWorking({
      oldContent: 'a\nb\n\nc\nd\n',
      newContent: 'a\nB\n\nC\nd\n',
      hunks: [hunk(2, 1, 2, 1), hunk(4, 1, 4, 1)],
    })
    const view = editorOf(wrapper)
    // "a\n" = 2 chars; "B\n" 2-4; "\n" 4-5; "C\n" 5-7.
    view.dispatch({ selection: EditorSelection.range(2, 7) })
    await flushPromises()

    await wrapper.get('[data-test="stage-selected-lines"]').trigger('click')
    await flushPromises()

    expect(Facade.StageLines).toHaveBeenCalledWith(
      '/repo',
      'a.txt',
      { old: [2, 4], new: [2, 4], fingerprint: 'fp' },
      false,
    )
  })

  it('ignoring whitespace offers no hunk or line actions, only a hint; showing whitespace brings them back', async () => {
    SettingsService.Get.mockResolvedValue(defaultSettingsFixture({ ignoreWhitespace: true }))
    await useSettingsStore().load()
    const wrapper = await mountWorking({
      oldContent: 'int a = 1;\n\nint b = 2;\nrest\n',
      newContent: 'int a = 10;\n        \nint b = 20;\nrest\n',
      hunks: [hunk(1, 1, 1, 1), hunk(3, 1, 3, 1)],
    })
    const controls = [
      'stage-hunk',
      'stage-selected-lines',
      'discard-hunk',
      'discard-selected-lines',
    ]

    for (const control of controls)
      expect(wrapper.find(`[data-test="${control}"]`).exists()).toBe(false)
    expect(wrapper.get('[data-test="whitespace-hint"]').text()).toContain('Show whitespace changes')

    await wrapper.get('[data-test="ignore-whitespace-toggle"]').trigger('click')
    await flushPromises()

    for (const control of controls)
      expect(wrapper.find(`[data-test="${control}"]`).exists()).toBe(true)
    expect(wrapper.find('[data-test="whitespace-hint"]').exists()).toBe(false)
  })

  it.each([
    [
      'an insertion: only the selected line',
      'a\nd\n',
      'a\nb\nc\nd\n',
      hunk(1, 0, 2, 2),
      'c',
      'c',
      [],
      [3],
    ],
    [
      'a one-for-one replacement: the line with its old one',
      'a\nb\nc\nd\n',
      'a\nB\nC\nd\n',
      hunk(2, 2, 2, 2),
      'C',
      'C',
      [3],
      [3],
    ],
    [
      'a last line given its newline: the line with its old one',
      'a\nb',
      'a\nb\n',
      hunk(2, 1, 2, 1),
      'b',
      'b',
      [2],
      [2],
    ],
    [
      'a hunk of several lines, selecting only some: just those lines',
      'line1\nA\nB\nC\nline5\n',
      'line1\nX\nY\nZ\nline5\n',
      hunk(2, 3, 2, 3),
      'Y',
      'Z',
      [3, 4],
      [3, 4],
    ],
    [
      'a replacement adding lines, selecting its last line: the whole hunk',
      'a\nb',
      'a\nb\nc\n',
      hunk(2, 1, 2, 2),
      'c',
      'c',
      [2],
      [2, 3],
    ],
    [
      'a replacement adding lines mid-file, selecting its first line: the whole hunk',
      'b\nq\nz\n',
      'b\nX\nQ\nz\n',
      hunk(2, 1, 2, 2),
      'X',
      'X',
      [2],
      [2, 3],
    ],
    [
      'a replacement removing lines: the whole hunk',
      'a\nx\ny\n',
      'a\nX',
      hunk(2, 2, 2, 1),
      'X',
      'X',
      [2, 3],
      [2],
    ],
  ])(
    'a line selection takes, per shown hunk it touches, %s, staged and discarded alike',
    async (_, oldContent, newContent, h, from, to, old, added) => {
      const wrapper = await mountWorking({ oldContent, newContent, hunks: [h] })
      const lines = { old, new: added, fingerprint: 'fp' }
      selectText(wrapper, from, to)
      await flushPromises()

      await wrapper.get('[data-test="stage-selected-lines"]').trigger('click')
      await flushPromises()
      expect(Facade.StageLines).toHaveBeenCalledWith('/repo', 'a.txt', lines, false)

      selectText(wrapper, from, to)
      await flushPromises()
      await wrapper.get('[data-test="discard-selected-lines"]').trigger('click')
      confirmOptions().accept()
      await flushPromises()
      expect(Facade.DiscardLines).toHaveBeenCalledWith('/repo', 'a.txt', lines)
    },
  )

  it('with a staged working-tree target, a hunk button reads "Unstage hunk" and calls StageLines reversed', async () => {
    const wrapper = await mountWorking(twoHunks, { staged: true })

    const buttons = wrapper.findAll('[data-test="stage-hunk"]')
    expect(buttons[0]!.text()).toBe('Unstage hunk')

    await buttons[0]!.trigger('click')
    await flushPromises()

    expect(Facade.StageLines).toHaveBeenCalledWith(
      '/repo',
      'a.txt',
      { old: [2], new: [2], fingerprint: 'fp' },
      true,
    )
  })

  const noControls = ['stage-hunk', 'stage-selected-lines']
  const addedFile = { oldContent: '', newContent: 'new\n', hunks: [hunk(0, 0, 1, 1)] }

  it.each([
    [
      'a renamed working file (status "R") gets no hunk controls',
      oneLine,
      { oldPath: 'old.txt', status: 'R' },
      noControls,
    ],
    [
      'an untracked working file (status "?") gets no hunk controls',
      addedFile,
      { status: '?' },
      noControls,
    ],
    [
      'a newly added staged working file (status "A") gets no hunk controls',
      addedFile,
      { staged: true, status: 'A' },
      noControls,
    ],
    [
      'a deleted working file (status "D") gets no hunk controls',
      { oldContent: 'old\n', newContent: '', hunks: [hunk(1, 1, 0, 0)] },
      { status: 'D' },
      noControls,
    ],
    [
      'a staged working-tree target renders no discard button',
      {
        oldContent: 'line1\nline2 old\nline3\n',
        newContent: 'line1\nline2 new\nline3\n',
        hunks: [hunk(2, 1, 2, 1)],
      },
      { staged: true },
      ['discard-hunk', 'discard-selected-lines'],
    ],
  ])('%s', async (_, file, working, absent) => {
    const wrapper = await mountWorking(file, working)

    for (const control of absent)
      expect(wrapper.find(`[data-test="${control}"]`).exists()).toBe(false)
  })

  it('"Stage selected lines" is disabled unless the selection falls entirely inside one change', async () => {
    const wrapper = await mountWorking(threeLines)

    // The default cursor position (0) sits in "line1", outside the chunk.
    expect(wrapper.get('[data-test="stage-selected-lines"]').attributes('disabled')).toBeDefined()

    const view = editorOf(wrapper)
    // "line1\n" = 6 chars; "X\n"=2 (6-8); "Y\n"=2 (8-10); "Z\n"=2 (10-12): inside the chunk.
    view.dispatch({ selection: EditorSelection.range(8, 12) })
    await flushPromises()

    expect(wrapper.get('[data-test="stage-selected-lines"]').attributes('disabled')).toBeUndefined()

    view.dispatch({ selection: EditorSelection.range(0, 0) }) // back outside the chunk
    await flushPromises()

    expect(wrapper.get('[data-test="stage-selected-lines"]').attributes('disabled')).toBeDefined()
  })

  it('with an unstaged working-tree target, "Discard hunk" confirms then calls Facade.DiscardLines and reloads', async () => {
    const wrapper = await mountWorking(twoHunks)

    const buttons = wrapper.findAll('[data-test="discard-hunk"]')
    expect(buttons.length).toBeGreaterThanOrEqual(2)
    expect(buttons[0]!.text()).toBe('Discard hunk')

    await buttons[0]!.trigger('click')

    expect(confirmRequire).toHaveBeenCalledTimes(1)
    const options = confirmOptions()
    expect(Facade.DiscardLines).not.toHaveBeenCalled()

    options.accept()
    await flushPromises()

    // The first hunk is the single-line substitution at line 2, the same
    // line number on both diff sides.
    expect(Facade.DiscardLines).toHaveBeenCalledTimes(1)
    expect(Facade.DiscardLines).toHaveBeenCalledWith('/repo', 'a.txt', {
      old: [2],
      new: [2],
      fingerprint: 'fp',
    })
    expect(Facade.Status).toHaveBeenCalledTimes(1)
    expect(Facade.DiffWorking).toHaveBeenCalledTimes(2)
  })

  // mountSideBySide mounts a.txt's commit diff in side-by-side mode.
  async function mountSideBySide(
    file: Partial<DiffFile> = oneLine,
    path = 'a.txt',
    oldPath = '',
  ): Promise<VueWrapper> {
    SettingsService.Get.mockResolvedValue(
      defaultSettingsFixture({ diffMode: DiffMode.SideBySideDiff }),
    )
    await useSettingsStore().load()
    Facade.DiffFile.mockResolvedValue(diffFileFixture(file))
    return mountCommit(path, oldPath)
  }

  it('side by side, scrolling side a sideways moves side b with it', async () => {
    const wrapper = await mountSideBySide()
    const [a, b] = wrapper.findAll<HTMLElement>('.cm-scroller').map((w) => w.element)

    a!.scrollLeft = 120
    a!.dispatchEvent(new Event('scroll'))

    expect(b!.scrollLeft).toBe(120)
  })

  it('side by side, scrolling side b up or down moves side a with it', async () => {
    const wrapper = await mountSideBySide()
    const [a, b] = wrapper.findAll<HTMLElement>('.cm-scroller').map((w) => w.element)

    b!.scrollTop = 240
    b!.dispatchEvent(new Event('scroll'))

    expect(a!.scrollTop).toBe(240)
  })

  it('a side the other could not move, already at that position, still leads when scrolled back to it', async () => {
    const wrapper = await mountSideBySide()
    const [a, b] = wrapper.findAll<HTMLElement>('.cm-scroller').map((w) => w.element)
    b!.scrollLeft = 50
    a!.scrollLeft = 50
    a!.dispatchEvent(new Event('scroll'))

    b!.scrollLeft = 60
    b!.dispatchEvent(new Event('scroll'))
    b!.scrollLeft = 50
    b!.dispatchEvent(new Event('scroll'))

    expect(a!.scrollLeft).toBe(50)
  })

  it('the echo scroll a synced write causes on the other side is ignored, so the two do not ping-pong', async () => {
    const wrapper = await mountSideBySide()
    const [a, b] = wrapper.findAll<HTMLElement>('.cm-scroller').map((w) => w.element)

    b!.scrollLeft = 80
    b!.dispatchEvent(new Event('scroll'))
    expect(a!.scrollLeft).toBe(80)

    a!.dispatchEvent(new Event('scroll')) // the browser's own echo of the write just made

    expect(a!.scrollLeft).toBe(80)
    expect(b!.scrollLeft).toBe(80)
  })

  it('switching to unified mode unties the old sides: their listeners are gone', async () => {
    const wrapper = await mountSideBySide()
    const [oldA, oldB] = wrapper.findAll<HTMLElement>('.cm-scroller').map((w) => w.element)

    await wrapper.get('[data-test="mode-toggle"]').trigger('click')
    await flushPromises()

    const scrollers = wrapper.findAll<HTMLElement>('.cm-scroller')
    expect(scrollers).toHaveLength(1)

    oldA!.scrollLeft = 99
    oldA!.dispatchEvent(new Event('scroll'))
    expect(oldB!.scrollLeft).toBe(0)
  })

  it('side by side shows the old text on side a and the new text on side b', async () => {
    const wrapper = await mountSideBySide()

    expect(wrapper.get('.cm-merge-a').text()).toContain('old')
    expect(wrapper.get('.cm-merge-b').text()).toContain('new')
  })

  it('every toolbar button has a title and aria-label; the mode and ignore-whitespace toggles expose aria-pressed', async () => {
    Facade.DiffFile.mockResolvedValue(diffFileFixture(oneLine))

    const wrapper = await mountCommit()

    const labels = {
      'mode-toggle': 'Switch to side-by-side',
      'ignore-whitespace-toggle': 'Showing whitespace',
      'prev-change': 'Previous change',
      'next-change': 'Next change',
      search: 'Search · Ctrl+F',
      'prev-file': 'Previous file',
      'next-file': 'Next file',
      'open-editor': 'Open in editor',
    }
    for (const [selector, label] of Object.entries(labels)) {
      const button = wrapper.get(`[data-test="${selector}"]`)
      expect(button.attributes('title')).toBe(label)
      expect(button.attributes('aria-label')).toBe(label)
    }
    expect(wrapper.get('[data-test="mode-toggle"]').attributes('aria-pressed')).toBe('false')
    expect(wrapper.get('[data-test="ignore-whitespace-toggle"]').attributes('aria-pressed')).toBe(
      'false',
    )

    await wrapper.get('[data-test="mode-toggle"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="mode-toggle"]').attributes('aria-pressed')).toBe('true')
  })

  describe('line numbers', () => {
    const grown = {
      oldContent: 'one\ntwo',
      newContent: 'zero\none\ntwo\nthree',
      hunks: [hunk(0, 0, 1, 1), hunk(3, 0, 4, 1)],
    }

    it('side by side, each side numbers its own version of the file', async () => {
      const wrapper = await mountSideBySide(grown)

      const [a, b] = editorViews(wrapper)
      expect(lineNumbersOf(a!)).toEqual(['1', '2'])
      expect(lineNumbersOf(b!)).toEqual(['1', '2', '3', '4'])
    })

    it('unified, the current lines are numbered', async () => {
      Facade.DiffFile.mockResolvedValue(diffFileFixture(grown))

      const wrapper = await mountCommit()

      expect(lineNumbersOf(editorOf(wrapper))).toEqual(['1', '2', '3', '4'])
    })

    // copyPathLine right-clicks the number of editor's first line and copies.
    async function copyPathLine(editor: EditorView): Promise<string[]> {
      const writeText = vi.fn()
      Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
      await new DOMWrapper(
        editor.dom.querySelector('.cm-lineNumbers .cm-gutterElement:nth-child(2)'),
      ).trigger('contextmenu')
      await clickMenuItem(new DOMWrapper(document.body), 'Copy path:line')
      return writeText.mock.calls.map(([text]) => String(text))
    }

    it('right-clicking a number offers "Copy path:line"', async () => {
      Facade.DiffFile.mockResolvedValue(diffFileFixture(grown))
      const wrapper = await mountCommit('src/a.txt')

      expect(await copyPathLine(editorOf(wrapper))).toEqual(['src/a.txt:1'])
    })

    it("side by side, a renamed file's left side copies the old path", async () => {
      const wrapper = await mountSideBySide(grown, 'new/a.txt', 'old/a.txt')

      const [a, b] = editorViews(wrapper)
      expect(await copyPathLine(a!)).toEqual(['old/a.txt:1'])
      expect(await copyPathLine(b!)).toEqual(['new/a.txt:1'])
    })
  })

  describe('search', () => {
    const fooTwice = {
      oldContent: 'foo\nbar\n',
      newContent: 'foo\nbar foo\nbaz\n',
      hunks: [hunk(2, 1, 2, 2)],
    }
    const ctrlF = { key: 'f', ctrlKey: true }

    // pressIn dispatches a keydown in view's content, as typing there does.
    function pressIn(view: EditorView, init: KeyboardEventInit): void {
      view.contentDOM.dispatchEvent(
        new KeyboardEvent('keydown', { ...init, bubbles: true, cancelable: true }),
      )
    }

    // searchesOpen says for each mounted editor whether its search is open.
    const searchesOpen = (wrapper: VueWrapper) =>
      editorViews(wrapper).map((v) => !!searchFieldOf(v))

    it("Ctrl+F outside the editor opens the unified diff's search; Enter steps through the matches and wraps, Shift+Enter steps back", async () => {
      Facade.DiffFile.mockResolvedValue(diffFileFixture(fooTwice))
      const wrapper = await mountCommit()
      const view = editorOf(wrapper)

      pressKey('f', ctrlF)
      searchFor(view, 'foo')
      await flushPromises()

      expect(matchesOf(view)).toEqual(['foo', 'foo'])
      const lines: number[] = []
      for (let i = 0; i < 3; i++) {
        pressEnterIn(view)
        lines.push(selectedLineOf(view))
      }
      pressEnterIn(view, true)
      lines.push(selectedLineOf(view))
      expect(lines).toEqual([1, 2, 1, 2])
    })

    it("side by side, Ctrl+F outside the editors opens the right side's search only, Ctrl+F in the left editor the left's", async () => {
      const wrapper = await mountSideBySide(fooTwice)
      const [a] = editorViews(wrapper)

      pressKey('f', ctrlF)
      expect(searchesOpen(wrapper)).toEqual([false, true])

      pressIn(a!, ctrlF)
      expect(searchesOpen(wrapper)).toEqual([true, true])
    })

    it("side by side, each toolbar button opens its side's search; the right one shows Ctrl+F", async () => {
      const wrapper = await mountSideBySide(fooTwice)

      await wrapper.get('[data-test="search-left"]').trigger('click')
      expect(searchesOpen(wrapper)).toEqual([true, false])
      await wrapper.get('[data-test="search-right"]').trigger('click')
      expect(searchesOpen(wrapper)).toEqual([true, true])

      expect(wrapper.get('[data-test="search-left"]').attributes('title')).toBe('Search left side')
      expect(wrapper.get('[data-test="search-right"]').attributes('title')).toBe(
        'Search right side · Ctrl+F',
      )
    })

    const removal = { oldContent: 'keep\ngone\n', newContent: 'keep\n', hunks: [hunk(2, 1, 1, 0)] }

    it('a removed line is found on the left side, not on the right', async () => {
      const wrapper = await mountSideBySide(removal)
      const [a, b] = editorViews(wrapper)
      await wrapper.get('[data-test="search-left"]').trigger('click')
      await wrapper.get('[data-test="search-right"]').trigger('click')

      searchFor(a!, 'gone')
      searchFor(b!, 'gone')
      await flushPromises()

      expect(matchesOf(a!)).toEqual(['gone'])
      expect(matchesOf(b!)).toEqual([])
    })

    it("the unified diff's search does not find a removed line", async () => {
      Facade.DiffFile.mockResolvedValue(diffFileFixture(removal))
      const wrapper = await mountCommit()
      const view = editorOf(wrapper)
      await wrapper.get('[data-test="search"]').trigger('click')

      searchFor(view, 'gone')
      await flushPromises()

      expect(wrapper.get('.cm-deletedChunk').text()).toContain('gone')
      expect(matchesOf(view)).toEqual([])
    })

    it('"match case" narrows the matches', async () => {
      Facade.DiffFile.mockResolvedValue(
        diffFileFixture({ oldContent: '', newContent: 'Foo foo\n', hunks: [hunk(0, 0, 1, 1)] }),
      )
      const wrapper = await mountCommit()
      const view = editorOf(wrapper)
      await wrapper.get('[data-test="search"]').trigger('click')
      searchFor(view, 'foo')
      await flushPromises()
      expect(matchesOf(view)).toEqual(['Foo', 'foo'])

      await new DOMWrapper(view.dom.querySelector('.cm-search input[name="case"]')).setValue(true)
      await flushPromises()

      expect(matchesOf(view)).toEqual(['foo'])
    })

    it('an open search keeps its text when the diff reloads with new content and when another file is shown', async () => {
      Facade.DiffWorking.mockResolvedValue(diffFileFixture(fooTwice))
      const wrapper = await mountWorking(fooTwice)
      await wrapper.get('[data-test="search"]').trigger('click')
      searchFor(editorOf(wrapper), 'foo')

      Facade.DiffWorking.mockResolvedValue(
        diffFileFixture({ ...fooTwice, newContent: 'foo\nbar foo\nfoo\n' }),
      )
      emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
      await flushPromises()
      expect(matchesOf(editorOf(wrapper))).toEqual(['foo', 'foo', 'foo'])

      await wrapper.setProps({ target: { ...workingProps().target, path: 'b.txt' } })
      await flushPromises()
      expect(searchFieldOf(editorOf(wrapper))?.value).toBe('foo')
    })

    it('a switch to unified keeps a search open on the left side only', async () => {
      const wrapper = await mountSideBySide(fooTwice)
      await wrapper.get('[data-test="search-left"]').trigger('click')
      searchFor(editorViews(wrapper)[0]!, 'foo')

      await wrapper.get('[data-test="mode-toggle"]').trigger('click')
      await flushPromises()

      expect(searchFieldOf(editorOf(wrapper))?.value).toBe('foo')
    })

    it('a reload puts the focus back in the search field only if it was there', async () => {
      Facade.DiffWorking.mockResolvedValue(diffFileFixture(fooTwice))
      const wrapper = mount(DiffView, {
        props: { ...base, ...workingProps() },
        attachTo: document.body,
      })
      await flushPromises()
      await wrapper.get('[data-test="search"]').trigger('click')
      const reload = async (newContent: string) => {
        Facade.DiffWorking.mockResolvedValue(diffFileFixture({ ...fooTwice, newContent }))
        emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
        await flushPromises()
      }

      searchFieldOf(editorOf(wrapper))!.focus()
      await reload('foo\n')
      expect(document.activeElement).toBe(searchFieldOf(editorOf(wrapper)))

      wrapper.get<HTMLButtonElement>('[data-test="open-editor"]').element.focus()
      await reload('foo\nfoo\n')
      expect(document.activeElement).toBe(wrapper.get('[data-test="open-editor"]').element)
      wrapper.unmount()
    })
  })
  describe('selected change', () => {
    // removalOnly removes line 2 and adds nothing in its place.
    const removalOnly = {
      oldContent: 'line1\nline2\nline3\n',
      newContent: 'line1\nline3\n',
      hunks: [hunk(2, 1, 1, 0)],
    }

    // selectedLinesOf are the numbers of view's lines outlined as the selected change.
    function selectedLinesOf(view: EditorView): number[] {
      return [...view.contentDOM.querySelectorAll('.cm-line.cm-chunk-selected')].map(
        (el) => view.state.doc.lineAt(view.posAtDOM(el)).number,
      )
    }

    const gutterMarksOf = (view: EditorView) =>
      view.dom.querySelectorAll('.cm-chunk-gutter-mark').length

    // removedBlockOf is the class list of the unified diff's removed block.
    const removedBlockOf = (view: EditorView) => [
      ...(view.contentDOM.querySelector('.cm-deletedChunk')?.classList ?? []),
    ]

    // click puts view's cursor at line n's start, as a mouse click does.
    function click(view: EditorView, n: number): void {
      view.dispatch({
        selection: { anchor: view.state.doc.line(n).from },
        userEvent: 'select.pointer',
      })
    }

    // sidesOf are side by side's old and new editors.
    function sidesOf(wrapper: VueWrapper): [EditorView, EditorView] {
      const [a, b] = editorViews(wrapper)
      if (!a || !b) throw new Error('not side by side')
      return [a, b]
    }

    const cursorLineOf = (view: EditorView) =>
      view.state.doc.lineAt(view.state.selection.main.head).number

    const step = async (wrapper: VueWrapper, button: 'next-change' | 'prev-change') => {
      await wrapper.get(`[data-test="${button}"]`).trigger('click')
      await flushPromises()
    }

    it('a diff opens with its first change selected and centred, the cursor left at the top; unified, the outline runs from its removed block through its last line', async () => {
      Facade.DiffFile.mockResolvedValue(diffFileFixture(twoHunks))
      const scroll = vi.spyOn(EditorView, 'scrollIntoView')
      const view = editorOf(await mountCommit())

      expect(selectedLinesOf(view)).toEqual([2])
      expect(scroll).toHaveBeenCalledWith(view.state.doc.line(2).from, { y: 'center' })
      scroll.mockRestore()
      expect(cursorLineOf(view)).toBe(1)
      expect(removedBlockOf(view)).toEqual(
        expect.arrayContaining(['cm-chunk-selected', 'cm-chunk-first']),
      )
      expect(removedBlockOf(view)).not.toContain('cm-chunk-last')
      const line = view.contentDOM.querySelector('.cm-line.cm-chunk-selected')!
      expect(line.classList).toContain('cm-chunk-last')
      expect(line.classList).not.toContain('cm-chunk-first')
      expect(gutterMarksOf(view)).toBe(2)
    })

    it('unified, a change of removed lines only is outlined and marked on its removed block', async () => {
      Facade.DiffFile.mockResolvedValue(diffFileFixture(removalOnly))
      const view = editorOf(await mountCommit())

      expect(selectedLinesOf(view)).toEqual([])
      expect(removedBlockOf(view)).toEqual(
        expect.arrayContaining(['cm-chunk-selected', 'cm-chunk-first', 'cm-chunk-last']),
      )
      expect(gutterMarksOf(view)).toBe(1)
    })

    it('side by side, a click in a change on the left selects it on both sides; a click on an unchanged line selects none', async () => {
      const wrapper = await mountSideBySide(twoHunks)
      const [a, b] = sidesOf(wrapper)
      expect([selectedLinesOf(a), selectedLinesOf(b)]).toEqual([[2], [2]])
      expect([gutterMarksOf(a), gutterMarksOf(b)]).toEqual([1, 1])

      click(a, 6)
      expect([selectedLinesOf(a), selectedLinesOf(b)]).toEqual([[6], [6]])

      click(b, 4)
      expect([selectedLinesOf(a), selectedLinesOf(b)]).toEqual([[], []])
      expect([gutterMarksOf(a), gutterMarksOf(b)]).toEqual([0, 0])
    })

    it('next and previous step from the selected change, or from the cursor when none is selected', async () => {
      Facade.DiffFile.mockResolvedValue(diffFileFixture(twoHunks))
      const wrapper = await mountCommit()
      const view = editorOf(wrapper)

      await step(wrapper, 'next-change')
      expect([selectedLinesOf(view), cursorLineOf(view)]).toEqual([[6], 6])
      await step(wrapper, 'prev-change')
      expect([selectedLinesOf(view), cursorLineOf(view)]).toEqual([[2], 2])

      click(view, 4)
      await step(wrapper, 'prev-change')
      expect(selectedLinesOf(view)).toEqual([2])
      click(view, 4)
      await step(wrapper, 'next-change')
      expect(selectedLinesOf(view)).toEqual([6])
    })

    it('side by side, a step from a change clicked on the left lands on the right side', async () => {
      const wrapper = await mountSideBySide(twoHunks)
      const [a, b] = sidesOf(wrapper)

      click(a, 4)
      await step(wrapper, 'next-change')

      expect([selectedLinesOf(a), selectedLinesOf(b)]).toEqual([[6], [6]])
      expect(cursorLineOf(b)).toBe(6)
    })

    it('a step scrolls the change to the middle of its pane', async () => {
      Facade.DiffFile.mockResolvedValue(diffFileFixture(twoHunks))
      const wrapper = await mountCommit()
      const scroll = vi.spyOn(EditorView, 'scrollIntoView')

      await step(wrapper, 'next-change')

      expect(scroll).toHaveBeenCalledWith(editorOf(wrapper).state.doc.line(6).from, {
        y: 'center',
      })
      scroll.mockRestore()
    })

    it('a switch between unified and side by side keeps the selection where the cursor was', async () => {
      Facade.DiffFile.mockResolvedValue(diffFileFixture(twoHunks))
      const wrapper = await mountCommit()
      await step(wrapper, 'next-change')

      await wrapper.get('[data-test="mode-toggle"]').trigger('click')
      await flushPromises()
      const [a, b] = sidesOf(wrapper)
      expect([selectedLinesOf(a), selectedLinesOf(b), cursorLineOf(b)]).toEqual([[6], [6], 6])

      await wrapper.get('[data-test="mode-toggle"]').trigger('click')
      await flushPromises()
      expect(selectedLinesOf(editorOf(wrapper))).toEqual([6])
    })

    it('a reload with new content keeps the selection where the cursor was; another file opens on its first change', async () => {
      const wrapper = await mountWorking(twoHunks)
      await step(wrapper, 'next-change')

      Facade.DiffWorking.mockResolvedValue(
        diffFileFixture({ ...twoHunks, newContent: twoHunks.newContent.replace('7', 'seven') }),
      )
      emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
      await flushPromises()
      expect(selectedLinesOf(editorOf(wrapper))).toEqual([6])

      await wrapper.setProps({ target: { ...workingProps().target, path: 'b.txt' } })
      await flushPromises()
      expect(selectedLinesOf(editorOf(wrapper))).toEqual([2])
    })

    it('a switch to unified keeps the place clicked on the left side', async () => {
      // Line 2 removed, line 4 changed: the new side's lines sit one up.
      const wrapper = await mountSideBySide({
        oldContent: 'line1\nline2\nline3\nline4 old\n',
        newContent: 'line1\nline3\nline4 new\n',
        hunks: [hunk(2, 1, 1, 0), hunk(4, 1, 3, 1)],
      })
      const toggle = async () => {
        await wrapper.get('[data-test="mode-toggle"]').trigger('click')
        await flushPromises()
      }
      const placeOf = (view: EditorView) => [selectedLinesOf(view), cursorLineOf(view)]

      click(sidesOf(wrapper)[0], 4)
      await toggle()
      expect(placeOf(editorOf(wrapper))).toEqual([[3], 3])

      await toggle()
      click(sidesOf(wrapper)[0], 3)
      await toggle()
      expect(placeOf(editorOf(wrapper))).toEqual([[], 2])
    })

    it('a file shown again after a binary file opens on its first change', async () => {
      const wrapper = await mountWorking(twoHunks)
      await step(wrapper, 'next-change')

      Facade.DiffWorking.mockResolvedValue(diffFileFixture({ binary: true }))
      await wrapper.setProps({ target: { ...workingProps().target, path: 'b.bin' } })
      await flushPromises()
      await wrapper.setProps(workingProps())
      await flushPromises()

      expect(selectedLinesOf(editorOf(wrapper))).toEqual([2])
    })
  })
})
