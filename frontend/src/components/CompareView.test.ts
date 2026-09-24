import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade, AppService } = vi.hoisted(() => ({
  Facade: { CompareCommits: vi.fn(), OpenFile: vi.fn(), LogIndexOf: vi.fn() },
  AppService: { OpenDiff: vi.fn() },
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade, AppService }))

import CompareView from './CompareView.vue'
import { useCompareStore } from '../stores/compare'
import { commitRow } from '../testing/fixtures/log-rows'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'

const row = (sha: string, shortSha: string) => commitRow({ sha, shortSha })

const props = { wt: '/repo' }

// mountComparing mounts CompareView comparing sha-a (older) with sha-b
// (newer), CompareCommits answering files.
async function mountComparing(files: ChangedFile[] = []) {
  Facade.CompareCommits.mockResolvedValue(files)
  const compare = useCompareStore()
  compare.toggleSelect('/repo', row('sha-a', 'aaa1111'), 7) // higher index: older
  compare.toggleSelect('/repo', row('sha-b', 'bbb2222'), 2) // lower index: newer
  const wrapper = mount(CompareView, { props })
  await flushPromises()
  return { wrapper, compare }
}

describe('CompareView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.CompareCommits.mockReset()
    AppService.OpenDiff.mockReset()
    Facade.OpenFile.mockReset().mockResolvedValue(undefined)
    Facade.LogIndexOf.mockReset().mockResolvedValue({ index: 0, found: false })
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

  it('FR-CMP-05 the from tag is labelled older with the older tone, the to tag newer', async () => {
    const { wrapper } = await mountComparing()

    const fromTag = wrapper.get('[data-test="from-tag"]')
    const toTag = wrapper.get('[data-test="to-tag"]')
    expect(fromTag.text()).toBe('older')
    expect(fromTag.classes()).toContain('tag-older')
    expect(toTag.text()).toBe('newer')
    expect(toTag.classes()).toContain('tag-newer')
  })

  it('FR-CMP-03 clicking swap reverses the comparison direction', async () => {
    const { wrapper } = await mountComparing()
    Facade.CompareCommits.mockClear()

    await wrapper.get('[data-test="swap"]').trigger('click')
    await flushPromises()

    expect(Facade.CompareCommits).toHaveBeenCalledWith('/repo', 'sha-b', 'sha-a')
    const text = wrapper.text()
    expect(text.indexOf('bbb2222')).toBeLessThan(text.indexOf('aaa1111'))
    expect(wrapper.get('[data-test="from-tag"]').text()).toBe('newer')
    expect(wrapper.get('[data-test="to-tag"]').text()).toBe('older')
  })

  it('FR-CMP-01 pressing Escape exits the comparison', async () => {
    const { compare } = await mountComparing()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))

    expect(compare.selection).toEqual([])
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
