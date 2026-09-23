import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { EditorSelection } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import { getChunks } from '@codemirror/merge'

const {
  mockDiffFile,
  mockDiffRange,
  mockDiffWorking,
  mockDiffAgainstWorking,
  mockSettingsGet,
  mockSettingsSet,
  mockLoadLog,
  mockLogRows,
  mockLogIndexOf,
  mockOpenFile,
  mockOpenFileFallback,
  mockStageLines,
  mockDiscardLines,
  mockStatus,
  mockRequire,
} = vi.hoisted(() => ({
  mockDiffFile: vi.fn(),
  mockDiffRange: vi.fn(),
  mockDiffWorking: vi.fn(),
  mockDiffAgainstWorking: vi.fn(),
  mockSettingsGet: vi.fn(),
  mockSettingsSet: vi.fn(),
  mockLoadLog: vi.fn(),
  mockLogRows: vi.fn(),
  mockLogIndexOf: vi.fn(),
  mockOpenFile: vi.fn(),
  mockOpenFileFallback: vi.fn(),
  mockStageLines: vi.fn(),
  mockDiscardLines: vi.fn(),
  mockStatus: vi.fn(),
  mockRequire: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: {
    DiffFile: mockDiffFile,
    DiffRange: mockDiffRange,
    DiffWorking: mockDiffWorking,
    DiffAgainstWorking: mockDiffAgainstWorking,
    LoadLog: mockLoadLog,
    LogRows: mockLogRows,
    CommitDetails: vi.fn(),
    LogIndexOf: mockLogIndexOf,
    OpenFile: mockOpenFile,
    OpenFileFallback: mockOpenFileFallback,
    StageLines: mockStageLines,
    DiscardLines: mockDiscardLines,
    Status: mockStatus,
  },
  SettingsService: { Get: mockSettingsGet, Set: mockSettingsSet },
}))

vi.mock('openvue/useconfirm', () => ({
  useConfirm: () => ({ require: mockRequire, close: vi.fn() }),
}))

vi.mock('@wailsio/runtime', () => ({
  Events: { On: vi.fn(() => () => {}) },
}))

import DiffView from './DiffView.vue'
import { Events } from '@wailsio/runtime'
import { useLogStore } from '../stores/log'
import { defaultSettingsFixture } from '../testing/fixtures/settings'
import { changedFilesFixture } from '../testing/fixtures/changes'

const settings = defaultSettingsFixture()

// open makes the log store show commit abc123 with paths, the first selected.
function open(...paths: string[]) {
  const log = useLogStore()
  const files = paths.map((path) => ({ path, status: 'M' }))
  log.details = {
    sha: 'abc123',
    parents: [],
    author: '',
    authorEmail: '',
    authorAt: 0,
    committer: '',
    committerEmail: '',
    committerAt: 0,
    body: '',
    files,
  }
  log.selectedFile = files[0]!
  return files
}

const props = { wt: '/repo' }

describe('DiffView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockDiffFile.mockReset()
    mockDiffRange.mockReset()
    mockDiffAgainstWorking.mockReset()
    mockDiffWorking.mockReset()
    mockSettingsGet.mockReset()
    mockSettingsSet.mockReset()
    mockSettingsGet.mockResolvedValue(settings)
    mockSettingsSet.mockResolvedValue(undefined)
    mockOpenFile.mockReset()
    mockOpenFile.mockResolvedValue(undefined)
    mockOpenFileFallback.mockReset()
    mockOpenFileFallback.mockResolvedValue(undefined)
    mockStageLines.mockReset()
    mockDiscardLines.mockReset()
    mockRequire.mockReset()
    mockStatus.mockReset()
    mockStatus.mockResolvedValue(changedFilesFixture())
    open('a.txt')
  })

  it('FR-DIFF-05 a .ts path gets syntax highlighting: a keyword token class appears', async () => {
    mockDiffFile.mockResolvedValue({
      path: 'a.ts',
      binary: false,
      tooLarge: false,
      oldContent: 'const x = 1\n',
      newContent: 'let x = 1\n',
    })
    open('a.ts')

    const wrapper = mount(DiffView, { props })
    await flushPromises()
    await flushPromises() // the language extension's own async load()

    expect(wrapper.find('.tok-keyword').exists()).toBe(true)
  })

  it('FR-DIFF-05 an unmatched filename renders plain text without throwing', async () => {
    mockDiffFile.mockResolvedValue({
      path: 'Makefile',
      binary: false,
      tooLarge: false,
      oldContent: 'old target\n',
      newContent: 'new target\n',
    })
    open('Makefile')

    const wrapper = mount(DiffView, { props })
    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain('old target')
    expect(wrapper.text()).toContain('new target')
  })

  it('FR-DIFF-06 toggling "ignore whitespace" clears a whitespace-only chunk', async () => {
    mockDiffFile.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'line1\nline2\nline3\n',
      newContent: 'line1\nline2   \nline3\n',
    })

    const wrapper = mount(DiffView, { props })
    await flushPromises()

    const before = EditorView.findFromDOM(wrapper.get('.cm-editor').element as HTMLElement)
    if (!before) throw new Error('no EditorView mounted')
    expect(getChunks(before.state)?.chunks.length).toBeGreaterThanOrEqual(1)

    await wrapper.get('[data-test="ignore-whitespace-toggle"]').trigger('click')
    await flushPromises()

    const after = EditorView.findFromDOM(wrapper.get('.cm-editor').element as HTMLElement)
    if (!after) throw new Error('no EditorView mounted')
    expect(getChunks(after.state)?.chunks.length ?? 0).toBe(0)
  })

  it('FR-DIFF-06 toggling "ignore whitespace" keeps a chunk that differs by a real word', async () => {
    mockDiffFile.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'line1\nline2\nline3\n',
      newContent: 'line1\nlineX\nline3\n',
    })

    const wrapper = mount(DiffView, { props })
    await flushPromises()

    await wrapper.get('[data-test="ignore-whitespace-toggle"]').trigger('click')
    await flushPromises()

    const view = EditorView.findFromDOM(wrapper.get('.cm-editor').element as HTMLElement)
    if (!view) throw new Error('no EditorView mounted')
    expect(getChunks(view.state)?.chunks.length).toBeGreaterThanOrEqual(1)
  })

  // Chunks are [fromA, toA, fromB, toB]; a chunk spans whole lines,
  // newline included, and the last line's toA/toB point one past the text.
  it.each([
    ['a short word swap', 'a b\n', 'a c\n', [[0, 4, 0, 4]]],
    ['a change at the text end after differing whitespace', 'x\na b', 'x\na  c', [[2, 6, 2, 7]]],
    ['a change followed by stripped trailing whitespace', 'a b  \nz\n', 'a c\nz\n', [[0, 6, 0, 4]]],
    ['an empty old text', '', 'a b\n', [[0, 0, 0, 4]]],
    ['an empty new text', 'a b\n', '', [[0, 4, 0, 0]]],
    ['two empty texts', '', '', []],
    ['CRLF lines differing only in whitespace', 'a b\r\nz\r\n', 'a  b\r\nz\r\n', []],
    ['CRLF lines with a real change', 'a b\r\nz\r\n', 'a c\r\nz\r\n', [[0, 4, 0, 4]]],
  ])('FR-DIFF-06 ignoring whitespace chunks exactly the changed lines: %s', async (_, oldContent, newContent, want) => {
    mockSettingsGet.mockResolvedValue(defaultSettingsFixture({ ignoreWhitespace: true }))
    mockDiffFile.mockResolvedValue({ path: 'a.txt', binary: false, tooLarge: false, oldContent, newContent })

    const wrapper = mount(DiffView, { props })
    await flushPromises()

    const view = EditorView.findFromDOM(wrapper.get('.cm-editor').element as HTMLElement)
    if (!view) throw new Error('no EditorView mounted')
    const chunks = getChunks(view.state)?.chunks ?? []
    expect(chunks.map((c) => [c.fromA, c.toA, c.fromB, c.toB])).toEqual(want)
  })

  it('FR-DIFF-06 ignoring whitespace in side-by-side mode renders a short word swap', async () => {
    mockSettingsGet.mockResolvedValue(defaultSettingsFixture({ ignoreWhitespace: true, diffMode: 'side-by-side' }))
    mockDiffFile.mockResolvedValue({ path: 'a.txt', binary: false, tooLarge: false, oldContent: 'a b\n', newContent: 'a c\n' })

    const wrapper = mount(DiffView, { props })
    await flushPromises()

    expect(wrapper.findAll('.cm-editor')).toHaveLength(2)
    expect(wrapper.text()).toContain('a b')
    expect(wrapper.text()).toContain('a c')
  })

  // repoChanged calls the component's own repo:changed handler.
  function repoChanged(scope: string): void {
    for (const [name, handler] of vi.mocked(Events.On).mock.calls as unknown as [string, (ev: unknown) => void][]) {
      if (name === 'repo:changed') handler({ data: { scope } })
    }
  }

  it('FR-REPO-04 a working-tree diff refetches on an outside change to the working tree', async () => {
    vi.mocked(Events.On).mockClear()
    mockDiffWorking.mockResolvedValue({ path: 'a.txt', binary: false, tooLarge: false, oldContent: 'a\n', newContent: 'b\n' })
    mount(DiffView, { props: { ...props, working: { path: 'a.txt', oldPath: '', staged: false, status: 'M' } } })
    await flushPromises()

    repoChanged('index')
    await flushPromises()

    expect(mockDiffWorking).toHaveBeenCalledTimes(2)
  })

  it("FR-REPO-04 a commit's diff does not refetch on a working-tree change", async () => {
    vi.mocked(Events.On).mockClear()
    mockDiffFile.mockResolvedValue({ path: 'a.txt', binary: false, tooLarge: false, oldContent: 'a\n', newContent: 'b\n' })
    mount(DiffView, { props })
    await flushPromises()

    repoChanged('index')
    await flushPromises()

    expect(mockDiffFile).toHaveBeenCalledTimes(1)
  })

  it('FR-DIFF-01 renders the computed diff in unified mode from real old/new content', async () => {
    mockDiffFile.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'shared line\nold only line\n',
      newContent: 'shared line\nnew only line\n',
    })

    const wrapper = mount(DiffView, { props })
    await flushPromises()

    expect(wrapper.text()).toContain('old only line')
    expect(wrapper.text()).toContain('new only line')
  })

  it('FR-DIFF-01 the mode toggle persists the choice and switches to side-by-side panes', async () => {
    mockDiffFile.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'shared line\nold only line\n',
      newContent: 'shared line\nnew only line\n',
    })

    const wrapper = mount(DiffView, { props })
    await flushPromises()
    expect(wrapper.findAll('.cm-editor')).toHaveLength(1)

    await wrapper.get('[data-test="mode-toggle"]').trigger('click')
    await flushPromises()

    expect(mockSettingsSet).toHaveBeenCalledTimes(1)
    expect(mockSettingsSet.mock.calls[0]?.[0]).toMatchObject({ diffMode: 'side-by-side' })
    expect(wrapper.findAll('.cm-editor')).toHaveLength(2)
  })

  it.each([
    ['binary', { binary: true, tooLarge: false }],
    ['too large', { binary: false, tooLarge: true }],
  ])('FR-DIFF-02 a %s file keeps Previous/Next file, so stepping through files goes on past it', async (_, flags) => {
    mockDiffFile.mockResolvedValue({ path: 'img.png', ...flags })
    open('img.png', 'b.txt')

    const wrapper = mount(DiffView, { props })
    await flushPromises()

    expect(wrapper.find('[data-test="prev-file"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="next-file"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="next-change"]').exists()).toBe(false)
  })

  it('FR-DIFF-03 shows a binary placeholder and mounts no CodeMirror container', async () => {
    mockDiffFile.mockResolvedValue({ path: 'img.png', binary: true, tooLarge: false })

    const wrapper = mount(DiffView, { props })
    await flushPromises()

    expect(wrapper.text().toLowerCase()).toContain('binary')
    expect(wrapper.find('.cm-editor').exists()).toBe(false)
  })

  it('FR-DIFF-04 shows a too-large placeholder, then loads the full diff on "Load anyway"', async () => {
    mockDiffFile.mockResolvedValueOnce({ path: 'big.txt', binary: false, tooLarge: true })

    const wrapper = mount(DiffView, { props })
    await flushPromises()

    expect(wrapper.text().toLowerCase()).toContain('too large')
    expect(wrapper.find('.cm-editor').exists()).toBe(false)

    mockDiffFile.mockResolvedValueOnce({
      path: 'big.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'old\n',
      newContent: 'new\n',
    })
    const loadAnyway = wrapper.findAll('button').find((b) => b.text() === 'Load anyway')
    await loadAnyway?.trigger('click')
    await flushPromises()

    expect(mockDiffFile).toHaveBeenLastCalledWith('/repo', 'abc123', 'a.txt', '', true)
    expect(wrapper.find('.cm-editor').exists()).toBe(true)
  })

  it('FR-DIFF-02 next/previous change moves the cursor between real chunks', async () => {
    mockDiffFile.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'line1\nline2 old\nline3\nline4\nline5\nline6 old\nline7\n',
      newContent: 'line1\nline2 new\nline3\nline4\nline5\nline6 new\nline7\n',
    })

    const wrapper = mount(DiffView, { props })
    await flushPromises()

    const view = EditorView.findFromDOM(wrapper.get('.cm-editor').element as HTMLElement)
    if (!view) throw new Error('no EditorView mounted')
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

  it('FR-DIFF-02 next file steps the log store to the next changed file and reloads', async () => {
    mockDiffFile.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'old\n',
      newContent: 'new\n',
    })

    const log = useLogStore()
    const files = open('a.txt', 'b.txt', 'c.txt')

    const wrapper = mount(DiffView, { props })
    await flushPromises()

    await wrapper.get('[data-test="next-file"]').trigger('click')
    await flushPromises()

    expect(log.selectedFile).toEqual(files[1])
    expect(mockDiffFile).toHaveBeenLastCalledWith('/repo', 'abc123', 'b.txt', '', false)
  })

  it('FR-CMP-02 with a range prop, loads via DiffRange and hides next/previous-file', async () => {
    mockDiffRange.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'old\n',
      newContent: 'new\n',
    })

    const wrapper = mount(DiffView, {
      props: { ...props, range: { from: 'sha-a', to: 'sha-b', path: 'a.txt', oldPath: '' } },
    })
    await flushPromises()

    expect(mockDiffRange).toHaveBeenCalledWith('/repo', 'sha-a', 'sha-b', 'a.txt', '', false)
    expect(mockDiffFile).not.toHaveBeenCalled()
    expect(wrapper.find('[data-test="next-file"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="prev-file"]').exists()).toBe(false)
  })

  it('FR-CMP-02 with range and files, next file steps the file list and emits update:range', async () => {
    mockDiffRange.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'old\n',
      newContent: 'new\n',
    })
    const files = [
      { path: 'a.txt', status: 'M' },
      { path: 'b.txt', status: 'M' },
    ]

    const wrapper = mount(DiffView, {
      props: { ...props, range: { from: 'sha-a', to: 'sha-b', path: 'a.txt', oldPath: '' }, files },
    })
    await flushPromises()

    expect(wrapper.find('[data-test="next-file"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="prev-file"]').exists()).toBe(true)

    await wrapper.get('[data-test="next-file"]').trigger('click')

    expect(wrapper.emitted('update:range')?.[0]).toEqual([
      { from: 'sha-a', to: 'sha-b', path: 'b.txt', oldPath: '' },
    ])

    await wrapper.setProps({ range: { from: 'sha-a', to: 'sha-b', path: 'b.txt', oldPath: '' } })
    await flushPromises()

    expect(mockDiffRange).toHaveBeenCalledWith('/repo', 'sha-a', 'sha-b', 'b.txt', '', false)
  })

  it('FR-CMP-02 with range and files, previous file from the first file is a no-op', async () => {
    mockDiffRange.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'old\n',
      newContent: 'new\n',
    })
    const files = [
      { path: 'a.txt', status: 'M' },
      { path: 'b.txt', status: 'M' },
    ]

    const wrapper = mount(DiffView, {
      props: { ...props, range: { from: 'sha-a', to: 'sha-b', path: 'a.txt', oldPath: '' }, files },
    })
    await flushPromises()

    await wrapper.get('[data-test="prev-file"]').trigger('click')

    expect(wrapper.emitted('update:range')).toBeUndefined()
  })

  it('FR-ED-09 "Open in editor" opens at the mounted view\'s current cursor line', async () => {
    mockDiffFile.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'line1\nline2\nline3\nline4\nline5\nline6\nline7\n',
      newContent: 'line1\nline2\nline3\nline4\nline5\nline6\nline7\n',
    })

    const wrapper = mount(DiffView, { props })
    await flushPromises()

    const view = EditorView.findFromDOM(wrapper.get('.cm-editor').element as HTMLElement)
    if (!view) throw new Error('no EditorView mounted')
    view.dispatch({ selection: EditorSelection.cursor(view.state.doc.line(5).from) })

    await wrapper.get('[data-test="open-editor"]').trigger('click')

    expect(mockOpenFile).toHaveBeenCalledWith('/repo', 'a.txt', 5)
  })

  it('FR-ED-09 in side-by-side mode, "Open in editor" opens at the new/right side\'s cursor line', async () => {
    mockDiffFile.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'line1\nline2\nline3\n',
      newContent: 'line1\nlineTWO\nlineTHREE\nline4\nline5\n',
    })

    const wrapper = mount(DiffView, { props })
    await flushPromises()
    await wrapper.get('[data-test="mode-toggle"]').trigger('click')
    await flushPromises()

    const editors = wrapper.findAll('.cm-editor')
    expect(editors).toHaveLength(2)
    const newView = EditorView.findFromDOM(editors[1]!.element as HTMLElement)
    if (!newView) throw new Error('no new-side EditorView mounted')
    newView.dispatch({ selection: EditorSelection.cursor(newView.state.doc.line(4).from) })

    await wrapper.get('[data-test="open-editor"]').trigger('click')

    expect(mockOpenFile).toHaveBeenCalledWith('/repo', 'a.txt', 4)
  })

  it('FR-ED-09 a deleted-lines-only region has no line of its own: the cursor lands on a real new-side line', async () => {
    mockDiffFile.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'line1\nline2\nline3\nline4\nline5\n',
      newContent: 'line1\nline5\n', // line2-4 deleted entirely, no new-side counterpart
    })

    const wrapper = mount(DiffView, { props })
    await flushPromises()

    const view = EditorView.findFromDOM(wrapper.get('.cm-editor').element as HTMLElement)
    if (!view) throw new Error('no EditorView mounted')
    // The line right after the deleted block: doc has no line for the
    // deletion itself to place a cursor on.
    view.dispatch({ selection: EditorSelection.cursor(view.state.doc.line(2).from) })

    await wrapper.get('[data-test="open-editor"]').trigger('click')

    expect(mockOpenFile).toHaveBeenCalledWith('/repo', 'a.txt', 2)
  })

  it('FR-ED-09 "Open in editor" with nothing mounted (too-large placeholder) opens at line 0', async () => {
    open('big.txt')
    mockDiffFile.mockResolvedValue({ path: 'big.txt', binary: false, tooLarge: true })

    const wrapper = mount(DiffView, { props })
    await flushPromises()

    await wrapper.get('[data-test="open-editor"]').trigger('click')

    expect(mockOpenFile).toHaveBeenCalledWith('/repo', 'big.txt', 0)
  })

  it('FR-ED-10 clicking "Open with git\'s editor" on an EDITOR_NOT_FOUND error calls Facade.OpenFileFallback', async () => {
    mockDiffFile.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'old\n',
      newContent: 'new\n',
    })
    mockOpenFile.mockRejectedValue(
      Object.assign(new Error('editor not found'), { cause: { message: 'no editor', code: 'EDITOR_NOT_FOUND' } }),
    )

    const wrapper = mount(DiffView, { props })
    await flushPromises()
    await wrapper.get('[data-test="open-editor"]').trigger('click')
    await flushPromises()

    const fallbackButton = wrapper
      .get('.error-banner')
      .findAll('button')
      .find((b) => b.text() === "Open with git's editor")
    await fallbackButton!.trigger('click')

    expect(mockOpenFileFallback).toHaveBeenCalledWith('/repo', 'a.txt', 1, 'git')
  })

  it('FR-ED-02 "Open in editor" calls Facade.OpenFile with the commit-mode file, in commit mode', async () => {
    mockDiffFile.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'old\n',
      newContent: 'new\n',
    })

    const wrapper = mount(DiffView, { props })
    await flushPromises()

    await wrapper.get('[data-test="open-editor"]').trigger('click')

    // The cursor's own default position (untouched): document start, line 1.
    expect(mockOpenFile).toHaveBeenCalledTimes(1)
    expect(mockOpenFile).toHaveBeenCalledWith('/repo', 'a.txt', 1)
  })

  it('FR-ED-02 "Open in editor" calls Facade.OpenFile with the range-mode file, in range mode', async () => {
    mockDiffRange.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'old\n',
      newContent: 'new\n',
    })

    const wrapper = mount(DiffView, {
      props: { ...props, range: { from: 'sha-a', to: 'sha-b', path: 'a.txt', oldPath: '' } },
    })
    await flushPromises()

    await wrapper.get('[data-test="open-editor"]').trigger('click')

    expect(mockOpenFile).toHaveBeenCalledTimes(1)
    expect(mockOpenFile).toHaveBeenCalledWith('/repo', 'a.txt', 1)
  })

  it('FR-CMP-04 with a workingCompare prop, loads via DiffAgainstWorking and hides next/previous-file', async () => {
    mockDiffAgainstWorking.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'old\n',
      newContent: 'new\n',
    })

    const wrapper = mount(DiffView, {
      props: { ...props, workingCompare: { sha: 'sha-a', path: 'a.txt', oldPath: '' } },
    })
    await flushPromises()

    expect(mockDiffAgainstWorking).toHaveBeenCalledWith('/repo', 'sha-a', 'a.txt', '', false)
    expect(mockDiffFile).not.toHaveBeenCalled()
    expect(wrapper.find('[data-test="prev-file"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="next-file"]').exists()).toBe(false)
  })

  it('FR-CHG-04 with a working prop, loads via DiffWorking and renders the fetched content', async () => {
    mockDiffWorking.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'shared line\nold only line\n',
      newContent: 'shared line\nnew only line\n',
    })

    const wrapper = mount(DiffView, {
      props: { ...props, working: { path: 'a.txt', oldPath: '', staged: true, status: 'M' } },
    })
    await flushPromises()

    expect(mockDiffWorking).toHaveBeenCalledTimes(1)
    expect(mockDiffWorking).toHaveBeenCalledWith('/repo', 'a.txt', '', true, false)
    expect(mockDiffFile).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('old only line')
    expect(wrapper.text()).toContain('new only line')
  })

  it('FR-CHG-04 with a working prop, hides next/previous-file but keeps next/previous-change', async () => {
    mockDiffWorking.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'old\n',
      newContent: 'new\n',
    })

    const wrapper = mount(DiffView, {
      props: { ...props, working: { path: 'a.txt', oldPath: '', staged: false, status: 'M' } },
    })
    await flushPromises()

    expect(wrapper.find('[data-test="prev-file"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="next-file"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="prev-change"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="next-change"]').exists()).toBe(true)
  })

  it('FR-CHG-03 with an unstaged working prop, clicking a hunk\'s "Stage hunk" button stages it and reloads', async () => {
    mockDiffWorking.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'line1\nline2 old\nline3\nline4\nline5\nline6 old\nline7\n',
      newContent: 'line1\nline2 new\nline3\nline4\nline5\nline6 new\nline7\n',
    })
    mockStageLines.mockResolvedValue(undefined)

    const wrapper = mount(DiffView, {
      props: { ...props, working: { path: 'a.txt', oldPath: '', staged: false, status: 'M' } },
    })
    await flushPromises()

    const buttons = wrapper.findAll('[data-test="stage-hunk"]')
    expect(buttons.length).toBeGreaterThanOrEqual(2)
    expect(buttons[0]!.text()).toBe('Stage hunk')

    await buttons[0]!.trigger('click')
    await flushPromises()

    // The first hunk is the single-line substitution at line 2, the same
    // line number on both diff sides.
    expect(mockStageLines).toHaveBeenCalledTimes(1)
    expect(mockStageLines).toHaveBeenCalledWith('/repo', 'a.txt', { old: [2], new: [2] }, false)
    expect(mockStatus).toHaveBeenCalledTimes(1)
    expect(mockDiffWorking).toHaveBeenCalledTimes(2)
  })

  it('FR-CHG-03 with a staged working prop, a hunk button reads "Unstage hunk" and calls StageLines reversed', async () => {
    mockDiffWorking.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'line1\nline2 old\nline3\nline4\nline5\nline6 old\nline7\n',
      newContent: 'line1\nline2 new\nline3\nline4\nline5\nline6 new\nline7\n',
    })
    mockStageLines.mockResolvedValue(undefined)

    const wrapper = mount(DiffView, {
      props: { ...props, working: { path: 'a.txt', oldPath: '', staged: true, status: 'M' } },
    })
    await flushPromises()

    const buttons = wrapper.findAll('[data-test="stage-hunk"]')
    expect(buttons[0]!.text()).toBe('Unstage hunk')

    await buttons[0]!.trigger('click')
    await flushPromises()

    expect(mockStageLines).toHaveBeenCalledWith('/repo', 'a.txt', { old: [2], new: [2] }, true)
  })

  it('FR-CHG-03 a renamed working file (status "R") gets no hunk controls', async () => {
    mockDiffWorking.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'old\n',
      newContent: 'new\n',
    })

    const wrapper = mount(DiffView, {
      props: { ...props, working: { path: 'a.txt', oldPath: 'old.txt', staged: false, status: 'R' } },
    })
    await flushPromises()

    expect(wrapper.find('[data-test="stage-hunk"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="stage-selected-lines"]').exists()).toBe(false)
  })

  it('FR-CHG-03 an untracked working file (status "?") gets no hunk controls', async () => {
    mockDiffWorking.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: '',
      newContent: 'new\n',
    })

    const wrapper = mount(DiffView, {
      props: { ...props, working: { path: 'a.txt', oldPath: '', staged: false, status: '?' } },
    })
    await flushPromises()

    expect(wrapper.find('[data-test="stage-hunk"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="stage-selected-lines"]').exists()).toBe(false)
  })

  it('FR-CHG-03 a newly added staged working file (status "A") gets no hunk controls', async () => {
    mockDiffWorking.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: '',
      newContent: 'new\n',
    })

    const wrapper = mount(DiffView, {
      props: { ...props, working: { path: 'a.txt', oldPath: '', staged: true, status: 'A' } },
    })
    await flushPromises()

    expect(wrapper.find('[data-test="stage-hunk"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="stage-selected-lines"]').exists()).toBe(false)
  })

  it('FR-CHG-03 a deleted working file (status "D") gets no hunk controls', async () => {
    mockDiffWorking.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'old\n',
      newContent: '',
    })

    const wrapper = mount(DiffView, {
      props: { ...props, working: { path: 'a.txt', oldPath: '', staged: false, status: 'D' } },
    })
    await flushPromises()

    expect(wrapper.find('[data-test="stage-hunk"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="stage-selected-lines"]').exists()).toBe(false)
  })

  it('FR-CHG-03 with a selection spanning some but not all of a hunk\'s changed lines, "Stage selected lines" sends exactly those line numbers', async () => {
    // One block-substitution hunk: old lines 2-4 (A,B,C) replaced by new
    // lines 2-4 (X,Y,Z). Selecting "Y\nZ" (new lines 3-4) should send
    // new: [3, 4], not the whole hunk's [2, 3, 4] — and no old-side lines,
    // since a unified-mode text selection can only land on new content.
    mockDiffWorking.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'line1\nA\nB\nC\nline5\n',
      newContent: 'line1\nX\nY\nZ\nline5\n',
    })
    mockStageLines.mockResolvedValue(undefined)

    const wrapper = mount(DiffView, {
      props: { ...props, working: { path: 'a.txt', oldPath: '', staged: false, status: 'M' } },
    })
    await flushPromises()

    const view = EditorView.findFromDOM(wrapper.get('.cm-editor').element as HTMLElement)
    if (!view) throw new Error('no EditorView mounted')
    // "line1\n" = 6 chars; "X\n"=2 (6-8); "Y\n"=2 (8-10); "Z\n"=2 (10-12).
    view.dispatch({ selection: EditorSelection.range(8, 12) })
    await flushPromises() // lets the disabled attribute clear before the click below

    await wrapper.get('[data-test="stage-selected-lines"]').trigger('click')
    await flushPromises()

    expect(mockStageLines).toHaveBeenCalledTimes(1)
    expect(mockStageLines).toHaveBeenCalledWith('/repo', 'a.txt', { old: [], new: [3, 4] }, false)
  })

  it('FR-CHG-03 "Stage selected lines" is disabled unless the selection falls entirely inside one chunk', async () => {
    mockDiffWorking.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'line1\nA\nB\nC\nline5\n',
      newContent: 'line1\nX\nY\nZ\nline5\n',
    })

    const wrapper = mount(DiffView, {
      props: { ...props, working: { path: 'a.txt', oldPath: '', staged: false, status: 'M' } },
    })
    await flushPromises()

    // The default cursor position (0) sits in "line1", outside the chunk.
    expect(wrapper.get('[data-test="stage-selected-lines"]').attributes('disabled')).toBeDefined()

    const view = EditorView.findFromDOM(wrapper.get('.cm-editor').element as HTMLElement)
    if (!view) throw new Error('no EditorView mounted')
    // "line1\n" = 6 chars; "X\n"=2 (6-8); "Y\n"=2 (8-10); "Z\n"=2 (10-12): inside the chunk.
    view.dispatch({ selection: EditorSelection.range(8, 12) })
    await flushPromises()

    expect(wrapper.get('[data-test="stage-selected-lines"]').attributes('disabled')).toBeUndefined()

    view.dispatch({ selection: EditorSelection.range(0, 0) }) // back outside the chunk
    await flushPromises()

    expect(wrapper.get('[data-test="stage-selected-lines"]').attributes('disabled')).toBeDefined()
  })

  it('FR-CHG-10 with an unstaged working prop, "Discard hunk" confirms then calls Facade.DiscardLines and reloads', async () => {
    mockDiffWorking.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'line1\nline2 old\nline3\nline4\nline5\nline6 old\nline7\n',
      newContent: 'line1\nline2 new\nline3\nline4\nline5\nline6 new\nline7\n',
    })
    mockDiscardLines.mockResolvedValue(undefined)

    const wrapper = mount(DiffView, {
      props: { ...props, working: { path: 'a.txt', oldPath: '', staged: false, status: 'M' } },
    })
    await flushPromises()

    const buttons = wrapper.findAll('[data-test="discard-hunk"]')
    expect(buttons.length).toBeGreaterThanOrEqual(2)
    expect(buttons[0]!.text()).toBe('Discard hunk')

    await buttons[0]!.trigger('click')

    expect(mockRequire).toHaveBeenCalledTimes(1)
    const options = mockRequire.mock.calls[0]![0] as { accept: () => void }
    expect(mockDiscardLines).not.toHaveBeenCalled()

    options.accept()
    await flushPromises()

    // The first hunk is the single-line substitution at line 2, the same
    // line number on both diff sides.
    expect(mockDiscardLines).toHaveBeenCalledTimes(1)
    expect(mockDiscardLines).toHaveBeenCalledWith('/repo', 'a.txt', { old: [2], new: [2] })
    expect(mockStatus).toHaveBeenCalledTimes(1)
    expect(mockDiffWorking).toHaveBeenCalledTimes(2)
  })

  it('FR-CHG-10 with a staged working prop, no discard button renders', async () => {
    mockDiffWorking.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'line1\nline2 old\nline3\n',
      newContent: 'line1\nline2 new\nline3\n',
    })

    const wrapper = mount(DiffView, {
      props: { ...props, working: { path: 'a.txt', oldPath: '', staged: true, status: 'M' } },
    })
    await flushPromises()

    expect(wrapper.find('[data-test="discard-hunk"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="discard-selected-lines"]').exists()).toBe(false)
  })

  it('FR-UX-09 every toolbar button has a title and aria-label; the mode and ignore-whitespace toggles expose aria-pressed', async () => {
    mockDiffFile.mockResolvedValue({
      path: 'a.txt',
      binary: false,
      tooLarge: false,
      oldContent: 'old\n',
      newContent: 'new\n',
    })

    const wrapper = mount(DiffView, { props })
    await flushPromises()

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
    expect(wrapper.get('[data-test="ignore-whitespace-toggle"]').attributes('aria-pressed')).toBe('false')

    await wrapper.get('[data-test="mode-toggle"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="mode-toggle"]').attributes('aria-pressed')).toBe('true')
  })
})
