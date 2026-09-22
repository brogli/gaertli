import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { EditorView } from '@codemirror/view'
import { getChunks } from '@codemirror/merge'

const {
  mockDiffFile,
  mockDiffRange,
  mockSettingsGet,
  mockSettingsSet,
  mockLoadLog,
  mockLogRows,
  mockLogIndexOf,
} = vi.hoisted(() => ({
  mockDiffFile: vi.fn(),
  mockDiffRange: vi.fn(),
  mockSettingsGet: vi.fn(),
  mockSettingsSet: vi.fn(),
  mockLoadLog: vi.fn(),
  mockLogRows: vi.fn(),
  mockLogIndexOf: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: {
    DiffFile: mockDiffFile,
    DiffRange: mockDiffRange,
    LoadLog: mockLoadLog,
    LogRows: mockLogRows,
    CommitDetails: vi.fn(),
    LogIndexOf: mockLogIndexOf,
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
    mockSettingsGet.mockReset()
    mockSettingsSet.mockReset()
    mockSettingsGet.mockResolvedValue(settings)
    mockSettingsSet.mockResolvedValue(undefined)
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
})
