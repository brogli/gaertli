import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const {
  mockCompareCommits,
  mockDiffRange,
  mockSettingsGet,
  mockSettingsSet,
  mockEventsOn,
  mockWindowClose,
  mockOpenFile,
} = vi.hoisted(() => ({
  mockCompareCommits: vi.fn(),
  mockDiffRange: vi.fn(),
  mockSettingsGet: vi.fn(),
  mockSettingsSet: vi.fn(),
  mockEventsOn: vi.fn(),
  mockWindowClose: vi.fn(),
  mockOpenFile: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { CompareCommits: mockCompareCommits, DiffRange: mockDiffRange, OpenFile: mockOpenFile },
  SettingsService: { Get: mockSettingsGet, Set: mockSettingsSet },
}))

vi.mock('@wailsio/runtime', () => ({
  Events: { On: mockEventsOn },
  Window: { Close: mockWindowClose },
}))

import CompareDiffView from './CompareDiffView.vue'

import { defaultSettingsFixture } from '../testing/fixtures/settings'
import { registeredHandler } from '../testing/events'

const files = [
  { path: 'a.txt', status: 'M' },
  { path: 'b.txt', status: 'M' },
]

const props = { wt: '/repo', from: 'sha-a', to: 'sha-b', path: 'a.txt', oldPath: '' }

function diffFor(path: string) {
  return { path, binary: false, tooLarge: false, oldContent: 'old\n', newContent: `new ${path}\n` }
}

describe('CompareDiffView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockCompareCommits.mockReset()
    mockCompareCommits.mockResolvedValue(files)
    mockDiffRange.mockReset()
    mockDiffRange.mockImplementation((_wt, _a, _b, path) => Promise.resolve(diffFor(path)))
    mockSettingsGet.mockReset()
    mockSettingsGet.mockResolvedValue(defaultSettingsFixture())
    mockSettingsSet.mockReset()
    mockSettingsSet.mockResolvedValue(undefined)
    mockEventsOn.mockReset()
    mockEventsOn.mockReturnValue(() => {})
    mockWindowClose.mockReset()
    mockOpenFile.mockReset()
    mockOpenFile.mockResolvedValue(undefined)
  })

  it('FR-CMP-02 loads the comparison and the named file, and renders the file list', async () => {
    const wrapper = mount(CompareDiffView, { props })
    await flushPromises()

    expect(mockCompareCommits).toHaveBeenCalledWith('/repo', 'sha-a', 'sha-b')
    expect(mockDiffRange).toHaveBeenCalledWith('/repo', 'sha-a', 'sha-b', 'a.txt', '', false)
    expect(wrapper.findAll('.files li')).toHaveLength(2)

    wrapper.unmount()
  })

  it('FR-CMP-02 clicking a different file in its own list loads that file', async () => {
    const wrapper = mount(CompareDiffView, { props })
    await flushPromises()
    mockDiffRange.mockClear()

    await wrapper.findAll('.files li')[1]!.trigger('click')
    await flushPromises()

    expect(mockDiffRange).toHaveBeenCalledWith('/repo', 'sha-a', 'sha-b', 'b.txt', '', false)

    wrapper.unmount()
  })

  it('FR-CMP-02 "Next file" steps to the following file', async () => {
    const wrapper = mount(CompareDiffView, { props })
    await flushPromises()
    mockDiffRange.mockClear()

    await wrapper.get('[data-test="next-file"]').trigger('click')
    await flushPromises()

    expect(mockDiffRange).toHaveBeenCalledWith('/repo', 'sha-a', 'sha-b', 'b.txt', '', false)

    wrapper.unmount()
  })

  it('FR-CMP-02 Escape closes the window', async () => {
    const wrapper = mount(CompareDiffView, { props })
    await flushPromises()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))

    expect(mockWindowClose).toHaveBeenCalledTimes(1)

    wrapper.unmount()
  })

  it('FR-CMP-02 a compare:show-file event switches the shown file', async () => {
    const wrapper = mount(CompareDiffView, { props })
    await flushPromises()
    mockDiffRange.mockClear()

    const handler = registeredHandler(mockEventsOn, 'compare:show-file')
    handler?.({ data: { path: 'b.txt', oldPath: '' } })
    await flushPromises()

    expect(mockDiffRange).toHaveBeenCalledWith('/repo', 'sha-a', 'sha-b', 'b.txt', '', false)

    wrapper.unmount()
  })

  it('FR-ED-02 an open-editor event from the file list calls Facade.OpenFile with wt, the path, and line 0', async () => {
    const wrapper = mount(CompareDiffView, { props })
    await flushPromises()

    await wrapper.findAll('[data-test="open-editor"]')[1]!.trigger('click')

    expect(mockOpenFile).toHaveBeenCalledTimes(1)
    expect(mockOpenFile).toHaveBeenCalledWith('/repo', 'b.txt', 0)

    wrapper.unmount()
  })
})
