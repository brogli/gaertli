import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade, AppService } = vi.hoisted(() => ({
  Facade: { CompareWithWorking: vi.fn(), OpenFile: vi.fn() },
  AppService: { OpenDiff: vi.fn() },
}))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade, AppService }))

import CompareWorkingView from '../CompareWorkingView.vue'
import { useCompareStore } from '@/stores/compare'
import { commitRow } from '@/testing/fixtures/log-rows'
import type { ChangedFile } from '../../../bindings/github.com/brogli/nebegit/internal/core'

const props = { wt: '/repo' }

// mountAgainstWorking mounts CompareWorkingView comparing sha-a with the
// working tree, CompareWithWorking answering files.
async function mountAgainstWorking(files: ChangedFile[] = []) {
  Facade.CompareWithWorking.mockResolvedValue(files)
  const compare = useCompareStore()
  compare.startAgainstWorking(commitRow({ sha: 'sha-a', shortSha: 'aaa1111' }))
  const wrapper = mount(CompareWorkingView, { props })
  await flushPromises()
  return { wrapper, compare }
}

describe('CompareWorkingView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.CompareWithWorking.mockReset()
    Facade.OpenFile.mockReset().mockResolvedValue(undefined)
    AppService.OpenDiff.mockReset()
  })

  it('FR-CMP-04 a click only selects a file; a double-click opens it in the diff window', async () => {
    const { wrapper } = await mountAgainstWorking([
      { path: 'b.txt', oldPath: 'a.txt', status: 'R' },
    ])
    const file = wrapper.get('.files li')

    await file.trigger('click')
    expect(file.classes()).toContain('selected')
    expect(AppService.OpenDiff).not.toHaveBeenCalled()

    await file.trigger('dblclick')
    expect(AppService.OpenDiff).toHaveBeenCalledWith('/repo', {
      kind: 'againstWorking',
      sha: 'sha-a',
      path: 'b.txt',
      oldPath: 'a.txt',
    })
  })

  it('FR-CMP-04 renders the changed files, with no swap button', async () => {
    const { wrapper } = await mountAgainstWorking([
      { path: 'a.txt', status: 'M' },
      { path: 'b.txt', status: 'A' },
    ])

    expect(Facade.CompareWithWorking).toHaveBeenCalledWith('/repo', 'sha-a')
    expect(wrapper.findAll('.files li')).toHaveLength(2)
    expect(wrapper.find('[data-test="swap"]').exists()).toBe(false)
  })

  it('FR-ED-02 an open-editor event from the file list calls Facade.OpenFile', async () => {
    const { wrapper } = await mountAgainstWorking([{ path: 'a.txt', status: 'M' }])

    await wrapper.get('[data-test="open-editor"]').trigger('click')

    expect(Facade.OpenFile).toHaveBeenCalledWith('/repo', 'a.txt', 0)
  })

  it('FR-CMP-04 notes that new, uncommitted files are not part of this comparison', async () => {
    const { wrapper } = await mountAgainstWorking()

    expect(wrapper.text()).toContain('untracked')
  })

  it("FR-CMP-04 shows the commit's short hash in the from colour box, like the two-commit comparison's", async () => {
    const { wrapper } = await mountAgainstWorking()

    const fromTag = wrapper.get('[data-test="from-tag"]')
    expect(fromTag.text()).toBe('aaa1111')
    expect(fromTag.attributes('title')).toBe('sha-a')
    expect(fromTag.classes()).toContain('from')
  })

  it('FR-CMP-04 clicking exit leaves the working-tree compare', async () => {
    const { wrapper, compare } = await mountAgainstWorking()

    await wrapper.get('[data-test="exit"]').trigger('click')

    expect(compare.workingCommit).toBeNull()
  })

  it('FR-UX-05 FR-CMP-04 pressing Escape leaves the working-tree compare', async () => {
    const { compare } = await mountAgainstWorking()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))

    expect(compare.workingCommit).toBeNull()
  })
})
