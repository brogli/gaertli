import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
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

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade,
  SettingsService,
}))

vi.mock('openvue/useconfirm', () => import('@/testing/confirm'))
import { confirmOptions, confirmRequire } from '@/testing/confirm'

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))
import { Events, emit } from '@/testing/runtime'

import DiffView from '../DiffView.vue'
import NoticeArea from '../NoticeArea.vue'
import { mountInBody } from '@/testing/mountInBody'
import { DiffKind } from '@/lib/diffTarget'
import { useSettingsStore } from '@/stores/settings'
import { defaultSettingsFixture } from '@/testing/fixtures/settings'
import { DiffMode } from '../../../bindings/github.com/brogli/nebegit/internal/settings'
import { changedFilesFixture } from '@/testing/fixtures/changes'
import { findButton } from '@/testing/findButton'
import { diffFileFixture, hunk } from '@/testing/fixtures/diff'
import {
  ChangeScope,
  Code,
  type DiffFile,
} from '../../../bindings/github.com/brogli/nebegit/internal/core'

const settings = defaultSettingsFixture()

// editorOf is the mounted unified view.
function editorOf(wrapper: VueWrapper): EditorView {
  const view = EditorView.findFromDOM(wrapper.get<HTMLElement>('.cm-editor').element)
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

// props mounts commit abc123's file, a.txt unless open picks another.
let props = commitProps('a.txt')

function open(path: string): void {
  props = commitProps(path)
}

function commitProps(path: string) {
  return { ...base, target: { kind: DiffKind.CommitDiff, sha: 'abc123', path, oldPath: '' } }
}

// mountCommit mounts props' commit diff and waits for it to load.
async function mountCommit(): Promise<VueWrapper> {
  const wrapper = mount(DiffView, { props })
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
    open('a.txt')
    await useSettingsStore().load()
  })

  it('FR-DIFF-05 a .ts path gets syntax highlighting: a keyword token class appears', async () => {
    Facade.DiffFile.mockResolvedValue(
      diffFileFixture({
        path: 'a.ts',
        oldContent: 'const x = 1\n',
        newContent: 'let x = 1\n',
        hunks: [hunk(1, 1, 1, 1)],
      }),
    )
    open('a.ts')

    const wrapper = await mountCommit()
    await flushPromises() // the language extension's own async load()

    expect(wrapper.find('.tok-keyword').exists()).toBe(true)
  })

  it('FR-DIFF-05 an unmatched filename renders plain text without throwing', async () => {
    Facade.DiffFile.mockResolvedValue(
      diffFileFixture({
        path: 'Makefile',
        oldContent: 'old target\n',
        newContent: 'new target\n',
        hunks: [hunk(1, 1, 1, 1)],
      }),
    )
    open('Makefile')

    const wrapper = await mountCommit()
    await flushPromises()

    expect(wrapper.text()).toContain('old target')
    expect(wrapper.text()).toContain('new target')
  })

  it('FR-DIFF-06 toggling "ignore whitespace" refetches through git and shows the hunks it returns', async () => {
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

    await wrapper.get('[data-test="ignore-whitespace-toggle"]').trigger('click')
    await flushPromises()

    expect(Facade.DiffFile).toHaveBeenLastCalledWith('/repo', 'abc123', 'a.txt', '', {
      force: false,
      ignoreWhitespace: true,
    })
    expect(chunkLines(wrapper)).toEqual([[3, 3]])
  })

  it('FR-DIFF-06 the whitespace toggle reads "Showing whitespace" while whitespace is shown', async () => {
    Facade.DiffFile.mockResolvedValue(diffFileFixture({ hunks: [hunk(1, 1, 1, 1)] }))

    const wrapper = await mountCommit()

    expect(wrapper.get('[data-test="ignore-whitespace-toggle"]').text()).toBe('Showing whitespace')
  })

  it('FR-DIFF-06 clicking the whitespace toggle saves ignore and it reads "Ignoring whitespace"', async () => {
    Facade.DiffFile.mockResolvedValue(diffFileFixture({ hunks: [hunk(1, 1, 1, 1)] }))
    const wrapper = await mountCommit()

    await wrapper.get('[data-test="ignore-whitespace-toggle"]').trigger('click')
    await flushPromises()

    expect(SettingsService.Set).toHaveBeenCalledWith(
      expect.objectContaining({ ignoreWhitespace: true }),
    )
    expect(wrapper.get('[data-test="ignore-whitespace-toggle"]').text()).toBe('Ignoring whitespace')
  })

  // loneCR is a diff whose line 3 holds a lone "\r"; git changes lines 5-6.
  const loneCR = diffFileFixture({
    oldContent: 'one\ntwo\nth\rree\nfour\nfive\nsix\n',
    newContent: 'one\ntwo\nth\rree\nfour\nFIVE\nSIX\n',
    hunks: [hunk(5, 2, 5, 2)],
    fingerprint: 'fp',
  })

  it("QA-COMP-01 a line holding a lone \\r stays one line, so git's hunks sit on git's lines", async () => {
    const wrapper = await mountWorking(loneCR)
    expect(wrapper.findAll('.cm-changedLine').map((l) => l.text())).toEqual(['FIVE', 'SIX'])

    const view = editorOf(wrapper)
    const text = view.state.doc.toString()
    view.dispatch({
      selection: EditorSelection.range(text.indexOf('FIVE'), text.indexOf('SIX') + 3),
    })
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

  it('QA-COMP-01 side by side, a line holding a lone \\r stays one line on both sides', async () => {
    SettingsService.Get.mockResolvedValue(
      defaultSettingsFixture({ diffMode: DiffMode.SideBySideDiff }),
    )
    await useSettingsStore().load()
    Facade.DiffFile.mockResolvedValue(loneCR)

    const wrapper = await mountCommit()

    for (const editor of wrapper.findAll<HTMLElement>('.cm-editor')) {
      const doc = EditorView.findFromDOM(editor.element)?.state.doc
      expect(doc?.line(3).text).toBe('th\rree')
      expect(doc?.lines).toBe(7)
    }
  })

  it('FR-DIFF-06 toggling "ignore whitespace" keeps the diff shown when git returns the same hunks', async () => {
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

  it('FR-REPO-04 a working-tree diff refetches on an outside change to the working tree', async () => {
    Events.On.mockClear()
    await mountWorking({ oldContent: 'a\n', newContent: 'b\n' })

    emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
    await flushPromises()

    expect(Facade.DiffWorking).toHaveBeenCalledTimes(2)
  })

  it('FR-CHG-03 a selection survives an outside-change refetch, then the diff window handing back the same target', async () => {
    // Same shape as the diff window: an outside-change refetch (fetch
    // returns a new but equal DiffFile) followed by the window handing
    // DiffView a new but equal target (a diff:show for the file already
    // shown), which reruns the target watch and reads the diff store's cache.
    const wrapper = await mountWorking({
      oldContent: 'line1\nA\nB\nC\nline5\n',
      newContent: 'line1\nX\nY\nZ\nline5\n',
      hunks: [hunk(2, 3, 2, 3)],
    })
    Facade.DiffWorking.mockImplementation(() =>
      Promise.resolve(
        diffFileFixture({
          fingerprint: 'fp',
          oldContent: 'line1\nA\nB\nC\nline5\n',
          newContent: 'line1\nX\nY\nZ\nline5\n',
          hunks: [hunk(2, 3, 2, 3)],
        }),
      ),
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

  it("FR-REPO-04 a commit's diff does not refetch on a working-tree change", async () => {
    Events.On.mockClear()
    Facade.DiffFile.mockResolvedValue(
      diffFileFixture({
        oldContent: 'a\n',
        newContent: 'b\n',
      }),
    )
    mount(DiffView, { props })
    await flushPromises()

    emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
    await flushPromises()

    expect(Facade.DiffFile).toHaveBeenCalledTimes(1)
  })

  it('FR-DIFF-01 renders the computed diff in unified mode from real old/new content', async () => {
    Facade.DiffFile.mockResolvedValue(
      diffFileFixture({
        oldContent: 'shared line\nold only line\n',
        newContent: 'shared line\nnew only line\n',
        hunks: [hunk(2, 1, 2, 1)],
      }),
    )

    const wrapper = await mountCommit()

    expect(wrapper.text()).toContain('old only line')
    expect(wrapper.text()).toContain('new only line')
  })

  it('FR-DIFF-01 the mode toggle persists the choice and switches to side-by-side panes', async () => {
    Facade.DiffFile.mockResolvedValue(
      diffFileFixture({
        oldContent: 'shared line\nold only line\n',
        newContent: 'shared line\nnew only line\n',
        hunks: [hunk(2, 1, 2, 1)],
      }),
    )

    const wrapper = await mountCommit()
    expect(wrapper.findAll('.cm-editor')).toHaveLength(1)

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
    'FR-DIFF-02 a %s file keeps Previous/Next file, so stepping through files goes on past it',
    async (_, flags) => {
      Facade.DiffFile.mockResolvedValue(diffFileFixture({ path: 'img.png', ...flags }))
      open('img.png')

      const wrapper = await mountCommit()

      expect(wrapper.find('[data-test="prev-file"]').exists()).toBe(true)
      expect(wrapper.find('[data-test="next-file"]').exists()).toBe(true)
      expect(wrapper.find('[data-test="next-change"]').exists()).toBe(false)
    },
  )

  it('FR-DIFF-03 shows a binary placeholder and mounts no CodeMirror container', async () => {
    Facade.DiffFile.mockResolvedValue(diffFileFixture({ path: 'img.png', binary: true }))

    const wrapper = await mountCommit()

    expect(wrapper.text().toLowerCase()).toContain('binary')
    expect(wrapper.find('.cm-editor').exists()).toBe(false)
  })

  it('FR-DIFF-04 shows a too-large placeholder, then loads the full diff on "Load anyway"', async () => {
    Facade.DiffFile.mockResolvedValueOnce(diffFileFixture({ path: 'big.txt', tooLarge: true }))

    const wrapper = await mountCommit()

    expect(wrapper.text().toLowerCase()).toContain('too large')
    expect(wrapper.find('.cm-editor').exists()).toBe(false)

    Facade.DiffFile.mockResolvedValueOnce(
      diffFileFixture({
        path: 'big.txt',
        oldContent: 'old\n',
        newContent: 'new\n',
        hunks: [hunk(1, 1, 1, 1)],
      }),
    )
    const loadAnyway = findButton(wrapper, 'Load anyway')
    await loadAnyway?.trigger('click')
    await flushPromises()

    expect(Facade.DiffFile).toHaveBeenLastCalledWith('/repo', 'abc123', 'a.txt', '', {
      force: true,
      ignoreWhitespace: false,
    })
    expect(wrapper.find('.cm-editor').exists()).toBe(true)
  })

  it('FR-DIFF-02 next/previous change moves the cursor between real changes', async () => {
    Facade.DiffFile.mockResolvedValue(
      diffFileFixture({
        oldContent: 'line1\nline2 old\nline3\nline4\nline5\nline6 old\nline7\n',
        newContent: 'line1\nline2 new\nline3\nline4\nline5\nline6 new\nline7\n',
        hunks: [hunk(2, 1, 2, 1), hunk(6, 1, 6, 1)],
      }),
    )

    const wrapper = await mountCommit()

    const view = editorOf(wrapper)
    const chunks = getChunks(view.state)?.chunks ?? []
    expect(chunks.length).toBeGreaterThanOrEqual(2)

    await wrapper.get('[data-test="next-change"]').trigger('click')
    const afterFirst = view.state.selection.main.head
    expect(afterFirst).toBeGreaterThanOrEqual(chunks[0]!.fromB)
    expect(afterFirst).toBeLessThanOrEqual(chunks[0]!.endB)

    await wrapper.get('[data-test="next-change"]').trigger('click')
    const afterSecond = view.state.selection.main.head
    expect(afterSecond).toBeGreaterThanOrEqual(chunks[1]!.fromB)
    expect(afterSecond).toBeLessThanOrEqual(chunks[1]!.endB)
    expect(afterSecond).not.toBe(afterFirst)

    await wrapper.get('[data-test="prev-change"]').trigger('click')
    const afterPrevious = view.state.selection.main.head
    expect(afterPrevious).toBe(afterFirst)
  })

  it('FR-DIFF-02 next/previous file ask for the neighbouring file', async () => {
    Facade.DiffFile.mockResolvedValue(
      diffFileFixture({
        oldContent: 'old\n',
        newContent: 'new\n',
        hunks: [hunk(1, 1, 1, 1)],
      }),
    )

    const wrapper = await mountCommit()
    await wrapper.get('[data-test="next-file"]').trigger('click')
    await wrapper.get('[data-test="prev-file"]').trigger('click')

    expect(wrapper.emitted('step-file')).toEqual([[1], [-1]])
  })

  it('FR-CMP-02 a range target loads via DiffRange', async () => {
    Facade.DiffRange.mockResolvedValue(
      diffFileFixture({
        oldContent: 'old\n',
        newContent: 'new\n',
        hunks: [hunk(1, 1, 1, 1)],
      }),
    )

    mount(DiffView, {
      props: {
        ...base,
        target: {
          kind: DiffKind.RangeDiff,
          from: 'sha-a',
          to: 'sha-b',
          path: 'a.txt',
          oldPath: '',
        },
      },
    })
    await flushPromises()

    expect(Facade.DiffRange).toHaveBeenCalledWith('/repo', 'sha-a', 'sha-b', 'a.txt', '', {
      force: false,
      ignoreWhitespace: false,
    })
    expect(Facade.DiffFile).not.toHaveBeenCalled()
  })

  it('FR-ED-09 "Open in editor" opens at the mounted view\'s current cursor line', async () => {
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

  it('FR-ED-09 in side-by-side mode, "Open in editor" opens at the new/right side\'s cursor line', async () => {
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

    const editors = wrapper.findAll<HTMLElement>('.cm-editor')
    expect(editors).toHaveLength(2)
    const newView = EditorView.findFromDOM(editors[1]!.element)
    if (!newView) throw new Error('no new-side EditorView mounted')
    newView.dispatch({ selection: EditorSelection.cursor(newView.state.doc.line(4).from) })

    await wrapper.get('[data-test="open-editor"]').trigger('click')

    expect(Facade.OpenFile).toHaveBeenCalledWith('/repo', 'a.txt', 4)
  })

  it('FR-ED-09 a deleted-lines-only region has no line of its own: the cursor lands on a real new-side line', async () => {
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

  it('FR-ED-09 "Open in editor" with nothing mounted (too-large placeholder) opens at line 0', async () => {
    open('big.txt')
    Facade.DiffFile.mockResolvedValue(diffFileFixture({ path: 'big.txt', tooLarge: true }))

    const wrapper = await mountCommit()

    await wrapper.get('[data-test="open-editor"]').trigger('click')

    expect(Facade.OpenFile).toHaveBeenCalledWith('/repo', 'big.txt', 0)
  })

  it('FR-ED-06 the toolbar\'s "Open in editor" failing EDITOR_NOT_FOUND shows a notice, the diff stays', async () => {
    Facade.DiffFile.mockResolvedValue(
      diffFileFixture({ oldContent: 'old\n', newContent: 'new\n', hunks: [hunk(1, 1, 1, 1)] }),
    )
    Facade.OpenFile.mockRejectedValueOnce(
      new Error('boom', { cause: { code: 'EDITOR_NOT_FOUND', message: '"myed" is not on PATH' } }),
    )
    const wrapper = await mountCommit()
    const { body: notice } = await mountInBody(NoticeArea)

    await wrapper.get('[data-test="open-editor"]').trigger('click')
    await flushPromises()

    expect(notice.text()).toContain('myed')
    expect(wrapper.find('.cm-editor').exists()).toBe(true)
    expect(wrapper.find('.error-banner').exists()).toBe(false)
  })

  it('FR-ED-02 "Open in editor" calls Facade.OpenFile with the commit-mode file, in commit mode', async () => {
    Facade.DiffFile.mockResolvedValue(
      diffFileFixture({
        oldContent: 'old\n',
        newContent: 'new\n',
        hunks: [hunk(1, 1, 1, 1)],
      }),
    )

    const wrapper = await mountCommit()

    await wrapper.get('[data-test="open-editor"]').trigger('click')

    // The cursor's own default position (untouched): document start, line 1.
    expect(Facade.OpenFile).toHaveBeenCalledTimes(1)
    expect(Facade.OpenFile).toHaveBeenCalledWith('/repo', 'a.txt', 1)
  })

  it('FR-ED-02 "Open in editor" calls Facade.OpenFile with the range-mode file, in range mode', async () => {
    Facade.DiffRange.mockResolvedValue(
      diffFileFixture({
        oldContent: 'old\n',
        newContent: 'new\n',
        hunks: [hunk(1, 1, 1, 1)],
      }),
    )

    const wrapper = mount(DiffView, {
      props: {
        ...base,
        target: {
          kind: DiffKind.RangeDiff,
          from: 'sha-a',
          to: 'sha-b',
          path: 'a.txt',
          oldPath: '',
        },
      },
    })
    await flushPromises()

    await wrapper.get('[data-test="open-editor"]').trigger('click')

    expect(Facade.OpenFile).toHaveBeenCalledTimes(1)
    expect(Facade.OpenFile).toHaveBeenCalledWith('/repo', 'a.txt', 1)
  })

  it('FR-CMP-04 an against-working target loads via DiffAgainstWorking', async () => {
    Facade.DiffAgainstWorking.mockResolvedValue(
      diffFileFixture({
        oldContent: 'old\n',
        newContent: 'new\n',
        hunks: [hunk(1, 1, 1, 1)],
      }),
    )

    mount(DiffView, {
      props: {
        ...base,
        target: { kind: DiffKind.AgainstWorkingDiff, sha: 'sha-a', path: 'a.txt', oldPath: '' },
      },
    })
    await flushPromises()

    expect(Facade.DiffAgainstWorking).toHaveBeenCalledWith('/repo', 'sha-a', 'a.txt', '', {
      force: false,
      ignoreWhitespace: false,
    })
    expect(Facade.DiffFile).not.toHaveBeenCalled()
  })

  it('FR-CHG-04 a working-tree target loads via DiffWorking and renders the fetched content', async () => {
    const wrapper = await mountWorking(
      {
        oldContent: 'shared line\nold only line\n',
        newContent: 'shared line\nnew only line\n',
        hunks: [hunk(2, 1, 2, 1)],
      },
      { staged: true },
    )

    expect(Facade.DiffWorking).toHaveBeenCalledTimes(1)
    expect(Facade.DiffWorking).toHaveBeenCalledWith('/repo', 'a.txt', '', true, {
      force: false,
      ignoreWhitespace: false,
    })
    expect(Facade.DiffFile).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('old only line')
    expect(wrapper.text()).toContain('new only line')
  })

  it('FR-UX-16 an outside change refetches the diff but keeps a LOCKED "Stage hunk"\'s Try again', async () => {
    const wrapper = await mountWorking({
      oldContent: 'a\nold\nb\n',
      newContent: 'a\nnew\nb\n',
      hunks: [hunk(2, 1, 2, 1)],
    })
    Facade.StageLines.mockRejectedValueOnce({ cause: { code: Code.Locked, message: 'locked' } })
    await wrapper.get('[data-test="stage-hunk"]').trigger('click')
    await flushPromises()
    expect(findButton(wrapper, 'Try again')).toBeTruthy()

    emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
    await flushPromises()

    expect(Facade.DiffWorking).toHaveBeenCalledTimes(2)
    expect(findButton(wrapper, 'Try again')).toBeTruthy()
  })

  it("FR-REPO-04 an outside change clears a line action's error that offers no retry, showing the diff again", async () => {
    const wrapper = await mountWorking({
      oldContent: 'a\nold\nb\n',
      newContent: 'a\nnew\nb\n',
      hunks: [hunk(2, 1, 2, 1)],
    })
    Facade.StageLines.mockRejectedValueOnce({
      cause: { code: Code.Internal, message: 'a.txt changed since it was shown' },
    })
    await wrapper.get('[data-test="stage-hunk"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="stage-hunk"]').exists()).toBe(false)

    emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
    await flushPromises()

    expect(wrapper.text()).not.toContain('changed since it was shown')
    expect(wrapper.find('[data-test="stage-hunk"]').exists()).toBe(true)
  })

  it('FR-CHG-03 with an unstaged working-tree target, clicking a hunk\'s "Stage hunk" button stages it and reloads', async () => {
    const wrapper = await mountWorking({
      oldContent: 'line1\nline2 old\nline3\nline4\nline5\nline6 old\nline7\n',
      newContent: 'line1\nline2 new\nline3\nline4\nline5\nline6 new\nline7\n',
      hunks: [hunk(2, 1, 2, 1), hunk(6, 1, 6, 1)],
    })

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
    ['a blank line replaced', 'a\n\nb\n', 'a\nX\nb\n', hunk(2, 1, 2, 1), [2], [2]],
    ['a line replaced by a blank one', 'a\nX\nb\n', 'a\n\nb\n', hunk(2, 1, 2, 1), [2], [2]],
    ['a lone blank line deleted', 'a\n\nb\n', 'a\nb\n', hunk(2, 1, 1, 0), [2], []],
  ])(
    'FR-CHG-03 "Stage hunk" on %s sends exactly git\'s hunk',
    async (_, oldContent, newContent, h, old, added) => {
      const wrapper = await mountWorking({ oldContent, newContent, hunks: [h] })

      await wrapper.get('[data-test="stage-hunk"]').trigger('click')
      await flushPromises()

      expect(Facade.StageLines).toHaveBeenCalledWith(
        '/repo',
        'a.txt',
        { old, new: added, fingerprint: 'fp' },
        false,
      )
    },
  )

  it('FR-CHG-10 "Discard hunk" on a blank line replaced sends exactly git\'s hunk', async () => {
    const wrapper = await mountWorking({
      oldContent: 'a\n\nb\n',
      newContent: 'a\nX\nb\n',
      hunks: [hunk(2, 1, 2, 1)],
    })

    await wrapper.get('[data-test="discard-hunk"]').trigger('click')
    confirmOptions().accept()
    await flushPromises()

    expect(Facade.DiscardLines).toHaveBeenCalledWith('/repo', 'a.txt', {
      old: [2],
      new: [2],
      fingerprint: 'fp',
    })
  })

  it('FR-CHG-03 "Stage selected lines" across hunks joined around a blank line sends only the lines git changed', async () => {
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

  it('FR-DIFF-06 ignoring whitespace offers no hunk or line actions, only a hint; showing whitespace brings them back', async () => {
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

  // selectText selects the new-side text from from's start to to's end.
  function selectText(wrapper: VueWrapper, from: string, to: string): void {
    const view = editorOf(wrapper)
    const text = view.state.doc.toString()
    view.dispatch({
      selection: EditorSelection.range(text.indexOf(from), text.indexOf(to) + to.length),
    })
  }

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
      'a replacement adding lines, selecting its first line: the whole hunk',
      'a\nb',
      'a\nb\nc\n',
      hunk(2, 1, 2, 2),
      'b',
      'b',
      [2],
      [2, 3],
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
      'a replacement adding lines, selecting all of it: the whole hunk',
      'a\nb',
      'a\nb\nc\n',
      hunk(2, 1, 2, 2),
      'b',
      'c',
      [2],
      [2, 3],
    ],
    [
      'a replacement adding lines elsewhere: the whole hunk',
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
    'FR-CHG-03 a line selection takes, per shown hunk it touches, %s',
    async (_, oldContent, newContent, h, from, to, old, added) => {
      const wrapper = await mountWorking({ oldContent, newContent, hunks: [h] })
      selectText(wrapper, from, to)
      await flushPromises()

      await wrapper.get('[data-test="stage-selected-lines"]').trigger('click')
      await flushPromises()

      expect(Facade.StageLines).toHaveBeenCalledWith(
        '/repo',
        'a.txt',
        { old, new: added, fingerprint: 'fp' },
        false,
      )
    },
  )

  it('FR-CHG-10 "Discard selected lines" in a replacement adding lines discards the whole hunk, restoring the old line', async () => {
    const wrapper = await mountWorking({
      oldContent: 'a\nb',
      newContent: 'a\nb\nc\n',
      hunks: [hunk(2, 1, 2, 2)],
    })
    selectText(wrapper, 'b', 'b')
    await flushPromises()

    await wrapper.get('[data-test="discard-selected-lines"]').trigger('click')
    confirmOptions().accept()
    await flushPromises()

    expect(Facade.DiscardLines).toHaveBeenCalledWith('/repo', 'a.txt', {
      old: [2],
      new: [2, 3],
      fingerprint: 'fp',
    })
  })

  it('FR-CHG-10 "Discard selected lines" on a modified line sends it as a whole change', async () => {
    const wrapper = await mountWorking({
      oldContent: 'a\nx\nc\n',
      newContent: 'a\nX\nc\n',
      hunks: [hunk(2, 1, 2, 1)],
    })
    selectText(wrapper, 'X', 'X')
    await flushPromises()

    await wrapper.get('[data-test="discard-selected-lines"]').trigger('click')
    confirmOptions().accept()
    await flushPromises()

    expect(Facade.DiscardLines).toHaveBeenCalledWith('/repo', 'a.txt', {
      old: [2],
      new: [2],
      fingerprint: 'fp',
    })
  })

  it('FR-CHG-03 the second git hunk\'s "Stage hunk" sends that hunk\'s old and new lines', async () => {
    const wrapper = await mountWorking({
      oldContent: 'l1\nl2\nl3\nl4\nl5\nl6\nl7\n',
      newContent: 'l1\nl3\nl4\nl5\nL6\nL6b\nl7\n',
      hunks: [hunk(2, 1, 1, 0), hunk(6, 1, 5, 2)],
    })

    const buttons = wrapper.findAll('[data-test="stage-hunk"]')
    expect(buttons).toHaveLength(2)
    await buttons[1]!.trigger('click')
    await flushPromises()

    expect(Facade.StageLines).toHaveBeenCalledWith(
      '/repo',
      'a.txt',
      { old: [6], new: [5, 6], fingerprint: 'fp' },
      false,
    )
  })

  it('FR-CHG-03 with a staged working-tree target, a hunk button reads "Unstage hunk" and calls StageLines reversed', async () => {
    const wrapper = await mountWorking(
      {
        oldContent: 'line1\nline2 old\nline3\nline4\nline5\nline6 old\nline7\n',
        newContent: 'line1\nline2 new\nline3\nline4\nline5\nline6 new\nline7\n',
        hunks: [hunk(2, 1, 2, 1), hunk(6, 1, 6, 1)],
      },
      { staged: true },
    )

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

  it('FR-CHG-03 a renamed working file (status "R") gets no hunk controls', async () => {
    const wrapper = await mountWorking(
      {
        oldContent: 'old\n',
        newContent: 'new\n',
        hunks: [hunk(1, 1, 1, 1)],
      },
      { oldPath: 'old.txt', status: 'R' },
    )

    expect(wrapper.find('[data-test="stage-hunk"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="stage-selected-lines"]').exists()).toBe(false)
  })

  it('FR-CHG-03 an untracked working file (status "?") gets no hunk controls', async () => {
    const wrapper = await mountWorking(
      {
        oldContent: '',
        newContent: 'new\n',
        hunks: [hunk(0, 0, 1, 1)],
      },
      { status: '?' },
    )

    expect(wrapper.find('[data-test="stage-hunk"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="stage-selected-lines"]').exists()).toBe(false)
  })

  it('FR-CHG-03 a newly added staged working file (status "A") gets no hunk controls', async () => {
    const wrapper = await mountWorking(
      {
        oldContent: '',
        newContent: 'new\n',
        hunks: [hunk(0, 0, 1, 1)],
      },
      { staged: true, status: 'A' },
    )

    expect(wrapper.find('[data-test="stage-hunk"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="stage-selected-lines"]').exists()).toBe(false)
  })

  it('FR-CHG-03 a deleted working file (status "D") gets no hunk controls', async () => {
    const wrapper = await mountWorking(
      {
        oldContent: 'old\n',
        newContent: '',
        hunks: [hunk(1, 1, 0, 0)],
      },
      { status: 'D' },
    )

    expect(wrapper.find('[data-test="stage-hunk"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="stage-selected-lines"]').exists()).toBe(false)
  })

  it('FR-CHG-03 with a selection spanning some but not all of a hunk\'s changed lines, "Stage selected lines" sends exactly those line numbers', async () => {
    // One block-substitution hunk: old lines 2-4 (A,B,C) replaced by new
    // lines 2-4 (X,Y,Z). Selecting "Y\nZ" (new lines 3-4) should send
    // new: [3, 4], not the whole hunk's [2, 3, 4], with the old lines they
    // replace one for one (B, C).
    const wrapper = await mountWorking({
      oldContent: 'line1\nA\nB\nC\nline5\n',
      newContent: 'line1\nX\nY\nZ\nline5\n',
      hunks: [hunk(2, 3, 2, 3)],
    })

    const view = editorOf(wrapper)
    // "line1\n" = 6 chars; "X\n"=2 (6-8); "Y\n"=2 (8-10); "Z\n"=2 (10-12).
    view.dispatch({ selection: EditorSelection.range(8, 12) })
    await flushPromises() // lets the disabled attribute clear before the click below

    await wrapper.get('[data-test="stage-selected-lines"]').trigger('click')
    await flushPromises()

    expect(Facade.StageLines).toHaveBeenCalledTimes(1)
    expect(Facade.StageLines).toHaveBeenCalledWith(
      '/repo',
      'a.txt',
      { old: [3, 4], new: [3, 4], fingerprint: 'fp' },
      false,
    )
  })

  it('FR-CHG-03 "Stage selected lines" is disabled unless the selection falls entirely inside one change', async () => {
    const wrapper = await mountWorking({
      oldContent: 'line1\nA\nB\nC\nline5\n',
      newContent: 'line1\nX\nY\nZ\nline5\n',
      hunks: [hunk(2, 3, 2, 3)],
    })

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

  it('FR-CHG-10 with an unstaged working-tree target, "Discard hunk" confirms then calls Facade.DiscardLines and reloads', async () => {
    const wrapper = await mountWorking({
      oldContent: 'line1\nline2 old\nline3\nline4\nline5\nline6 old\nline7\n',
      newContent: 'line1\nline2 new\nline3\nline4\nline5\nline6 new\nline7\n',
      hunks: [hunk(2, 1, 2, 1), hunk(6, 1, 6, 1)],
    })

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

  it('FR-CHG-10 with a staged working-tree target, no discard button renders', async () => {
    const wrapper = await mountWorking(
      {
        oldContent: 'line1\nline2 old\nline3\n',
        newContent: 'line1\nline2 new\nline3\n',
        hunks: [hunk(2, 1, 2, 1)],
      },
      { staged: true },
    )

    expect(wrapper.find('[data-test="discard-hunk"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="discard-selected-lines"]').exists()).toBe(false)
  })

  // mountSideBySide mounts a.txt's commit diff in side-by-side mode.
  async function mountSideBySide(): Promise<VueWrapper> {
    SettingsService.Get.mockResolvedValue(
      defaultSettingsFixture({ diffMode: DiffMode.SideBySideDiff }),
    )
    await useSettingsStore().load()
    Facade.DiffFile.mockResolvedValue(
      diffFileFixture({ oldContent: 'old\n', newContent: 'new\n', hunks: [hunk(1, 1, 1, 1)] }),
    )
    return mountCommit()
  }

  it('FR-DIFF-08 side by side, scrolling side a sideways moves side b with it', async () => {
    const wrapper = await mountSideBySide()
    const [a, b] = wrapper.findAll<HTMLElement>('.cm-scroller').map((w) => w.element)

    a!.scrollLeft = 120
    a!.dispatchEvent(new Event('scroll'))

    expect(b!.scrollLeft).toBe(120)
  })

  it('FR-DIFF-08 the echo scroll a synced write causes on the other side is ignored, so the two do not ping-pong', async () => {
    const wrapper = await mountSideBySide()
    const [a, b] = wrapper.findAll<HTMLElement>('.cm-scroller').map((w) => w.element)

    b!.scrollLeft = 80
    b!.dispatchEvent(new Event('scroll'))
    expect(a!.scrollLeft).toBe(80)

    a!.dispatchEvent(new Event('scroll')) // the browser's own echo of the write just made

    expect(a!.scrollLeft).toBe(80)
    expect(b!.scrollLeft).toBe(80)
  })

  it('FR-DIFF-08 switching to unified mode unties the old sides: their listeners are gone', async () => {
    const wrapper = await mountSideBySide()
    const [oldA, oldB] = wrapper.findAll<HTMLElement>('.cm-scroller').map((w) => w.element)

    await wrapper.get('[data-test="mode-toggle"]').trigger('click')
    await flushPromises()

    const scrollers = wrapper.findAll<HTMLElement>('.cm-scroller')
    expect(scrollers).toHaveLength(1)
    expect(() => scrollers[0]!.element.dispatchEvent(new Event('scroll'))).not.toThrow()

    oldA!.scrollLeft = 99
    oldA!.dispatchEvent(new Event('scroll'))
    expect(oldB!.scrollLeft).toBe(0)
  })

  it('FR-DIFF-09 side by side restores its saved sides splitter sizes, side a and b each in their own panel', async () => {
    localStorage.setItem('nebegit.layout.diffSides', JSON.stringify([40, 60]))

    const wrapper = await mountSideBySide()

    const panels = wrapper.findAll<HTMLElement>('.p-splitterpanel')
    expect(panels).toHaveLength(2)
    expect(panels[0]!.element.style.flexBasis).toContain('40%')
    expect(panels[0]!.find('.cm-merge-a').exists()).toBe(true)
    expect(panels[1]!.find('.cm-merge-b').exists()).toBe(true)
    expect(wrapper.get('.cm-mergeView').find('.cm-merge-a').exists()).toBe(true)
    expect(wrapper.get('.cm-mergeView').find('.cm-merge-b').exists()).toBe(true)
  })

  it('FR-DIFF-09 unified mode has no sides splitter, one editor', async () => {
    Facade.DiffFile.mockResolvedValue(
      diffFileFixture({ oldContent: 'old\n', newContent: 'new\n', hunks: [hunk(1, 1, 1, 1)] }),
    )

    const wrapper = await mountCommit()

    expect(wrapper.find('.p-splitter').exists()).toBe(false)
    expect(wrapper.findAll('.cm-editor')).toHaveLength(1)
  })

  it('FR-UX-09 every toolbar button has a title and aria-label; the mode and ignore-whitespace toggles expose aria-pressed', async () => {
    Facade.DiffFile.mockResolvedValue(
      diffFileFixture({
        oldContent: 'old\n',
        newContent: 'new\n',
        hunks: [hunk(1, 1, 1, 1)],
      }),
    )

    const wrapper = await mountCommit()

    for (const selector of [
      'mode-toggle',
      'ignore-whitespace-toggle',
      'prev-change',
      'next-change',
      'prev-file',
      'next-file',
      'open-editor',
    ]) {
      const button = wrapper.get(`[data-test="${selector}"]`)
      expect(button.attributes('title')).toBeTruthy()
      expect(button.attributes('aria-label')).toBeTruthy()
    }
    expect(wrapper.get('[data-test="mode-toggle"]').attributes('aria-pressed')).toBe('false')
    expect(wrapper.get('[data-test="ignore-whitespace-toggle"]').attributes('aria-pressed')).toBe(
      'false',
    )

    await wrapper.get('[data-test="mode-toggle"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="mode-toggle"]').attributes('aria-pressed')).toBe('true')
  })
})
