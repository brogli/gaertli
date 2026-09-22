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
  mockSettingsGet,
  mockSettingsSet,
  mockLoadLog,
  mockLogRows,
  mockLogIndexOf,
  mockOpenFile,
  mockStageLines,
  mockStatus,
} = vi.hoisted(() => ({
  mockDiffFile: vi.fn(),
  mockDiffRange: vi.fn(),
  mockDiffWorking: vi.fn(),
  mockSettingsGet: vi.fn(),
  mockSettingsSet: vi.fn(),
  mockLoadLog: vi.fn(),
  mockLogRows: vi.fn(),
  mockLogIndexOf: vi.fn(),
  mockOpenFile: vi.fn(),
  mockStageLines: vi.fn(),
  mockStatus: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: {
    DiffFile: mockDiffFile,
    DiffRange: mockDiffRange,
    DiffWorking: mockDiffWorking,
    LoadLog: mockLoadLog,
    LogRows: mockLogRows,
    CommitDetails: vi.fn(),
    LogIndexOf: mockLogIndexOf,
    OpenFile: mockOpenFile,
    StageLines: mockStageLines,
    Status: mockStatus,
  },
  SettingsService: { Get: mockSettingsGet, Set: mockSettingsSet },
}))

vi.mock('@wailsio/runtime', () => ({
  Events: { On: vi.fn(() => () => {}) },
}))

import DiffView from './DiffView.vue'
import { useLogStore } from '../stores/log'
import { defaultSettingsFixture } from '../testing/fixtures/settings'

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
    mockDiffWorking.mockReset()
    mockSettingsGet.mockReset()
    mockSettingsSet.mockReset()
    mockSettingsGet.mockResolvedValue(settings)
    mockSettingsSet.mockResolvedValue(undefined)
    mockOpenFile.mockReset()
    mockOpenFile.mockResolvedValue(undefined)
    mockStageLines.mockReset()
    mockStatus.mockReset()
    mockStatus.mockResolvedValue({ staged: [], unstaged: [], untracked: [], conflicted: [] })
    open('a.txt')
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

    expect(mockOpenFile).toHaveBeenCalledTimes(1)
    expect(mockOpenFile).toHaveBeenCalledWith('/repo', 'a.txt', 0)
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
    expect(mockOpenFile).toHaveBeenCalledWith('/repo', 'a.txt', 0)
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
})
