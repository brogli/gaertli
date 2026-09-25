import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade, AppService } = vi.hoisted(() => ({
  Facade: { CompareCommits: vi.fn(), OpenFile: vi.fn() },
  AppService: { OpenDiff: vi.fn() },
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade, AppService }))

import CompareView from './CompareView.vue'
import { useLogStore } from '../stores/log'
import { useCompareStore } from '../stores/compare'
import { commitRow } from '../testing/fixtures/log-rows'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'

const row = (sha: string, shortSha: string) => commitRow({ sha, shortSha })

const props = { wt: '/repo' }

// mountComparing mounts CompareView comparing sha-a (selected first: from)
// with sha-b (selected second: to), CompareCommits answering files.
async function mountComparing(files: ChangedFile[] = []) {
  Facade.CompareCommits.mockResolvedValue(files)
  const log = useLogStore()
  const compare = useCompareStore()
  log.selection = [row('sha-a', 'aaa1111'), row('sha-b', 'bbb2222')]
  const wrapper = mount(CompareView, { props })
  await flushPromises()
  return { wrapper, log, compare }
}

describe('CompareView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    Facade.CompareCommits.mockReset()
    AppService.OpenDiff.mockReset()
    Facade.OpenFile.mockReset().mockResolvedValue(undefined)
  })

  it('FR-CMP-01, FR-CMP-03 shows both commits short SHAs in from->to order and the changed files', async () => {
    const { wrapper } = await mountComparing([
      { path: 'a.txt', status: 'M' },
      { path: 'b.txt', status: 'A' },
    ])

    expect(Facade.CompareCommits).toHaveBeenCalledWith('/repo', 'sha-a', 'sha-b')
    const text = wrapper.text()
    expect(text.indexOf('aaa1111')).toBeLessThan(text.indexOf('bbb2222'))
    expect(wrapper.findAll('.files li')).toHaveLength(2)
  })

  it("FR-CMP-05 each side's box holds its short hash in its colour, not older or newer", async () => {
    const { wrapper } = await mountComparing()

    const fromTag = wrapper.get('[data-test="from-tag"]')
    const toTag = wrapper.get('[data-test="to-tag"]')
    expect(fromTag.text()).toBe('aaa1111')
    expect(fromTag.attributes('title')).toBe('sha-a')
    expect(fromTag.classes()).toContain('from')
    expect(toTag.text()).toBe('bbb2222')
    expect(toTag.attributes('title')).toBe('sha-b')
    expect(toTag.classes()).toContain('to')
    expect(wrapper.get('header').text()).not.toMatch(/older|newer/)
  })

  it('FR-CMP-03 clicking swap reverses the comparison direction; the boxes keep their colours, the hashes in them swap', async () => {
    const { wrapper } = await mountComparing()
    Facade.CompareCommits.mockClear()

    await wrapper.get('[data-test="swap"]').trigger('click')
    await flushPromises()

    expect(Facade.CompareCommits).toHaveBeenCalledWith('/repo', 'sha-b', 'sha-a')
    const fromTag = wrapper.get('[data-test="from-tag"]')
    const toTag = wrapper.get('[data-test="to-tag"]')
    expect(fromTag.text()).toBe('bbb2222')
    expect(fromTag.classes()).toContain('from')
    expect(toTag.text()).toBe('aaa1111')
    expect(toTag.classes()).toContain('to')
  })

  it('FR-CMP-01 pressing Escape exits the comparison', async () => {
    const { log } = await mountComparing()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))

    expect(log.selection).toEqual([])
  })

  it('FR-CMP-02 a click only selects a file; a double-click opens it in the diff window', async () => {
    const { wrapper } = await mountComparing([
      { path: 'a.txt', status: 'M' },
      { path: 'b.txt', oldPath: 'a2.txt', status: 'R' },
    ])

    const file = wrapper.findAll('.files li')[1]!
    await file.trigger('click')
    expect(file.classes()).toContain('selected')
    expect(AppService.OpenDiff).not.toHaveBeenCalled()

    await file.trigger('dblclick')

    expect(AppService.OpenDiff).toHaveBeenCalledWith('/repo', {
      kind: 'range',
      from: 'sha-a',
      to: 'sha-b',
      path: 'b.txt',
      oldPath: 'a2.txt',
    })
  })

  it("FR-LOG-16 a comparison's files switch to the folder tree, where a double-click opens a file", async () => {
    const { wrapper } = await mountComparing([{ path: 'src/b.txt', status: 'M' }])

    await wrapper.get('[data-test="file-tree-toggle"]').trigger('click')
    await wrapper
      .get('[role="treeitem"][aria-label="b.txt"] [data-pc-section="nodecontent"]')
      .trigger('dblclick')

    expect(AppService.OpenDiff).toHaveBeenCalledWith('/repo', {
      kind: 'range',
      from: 'sha-a',
      to: 'sha-b',
      path: 'src/b.txt',
      oldPath: '',
    })
  })

  it('FR-ED-02 an open-editor event from the file list calls Facade.OpenFile with wt, the path, and line 0', async () => {
    const { wrapper } = await mountComparing([
      { path: 'a.txt', status: 'M' },
      { path: 'b.txt', status: 'A' },
    ])

    await wrapper.findAll('[data-test="open-editor"]')[1]!.trigger('click')

    expect(Facade.OpenFile).toHaveBeenCalledTimes(1)
    expect(Facade.OpenFile).toHaveBeenCalledWith('/repo', 'b.txt', 0)
  })
})
