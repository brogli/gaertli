import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({ Facade: { PopStash: vi.fn(), DropStash: vi.fn() } }))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

vi.mock('openvue/useconfirm', () => import('../testing/confirm'))
import { confirmOptions, confirmRequire } from '../testing/confirm'

import StashBanner from './StashBanner.vue'
import { useRepoStore } from '../stores/repo'
import { useChangesStore } from '../stores/changes'
import { changedFilesFixture } from '../testing/fixtures/changes'
import { findButton } from '../testing/findButton'
import { summaryFixture } from '../testing/fixtures/summary'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'

// mountWithStash mounts StashBanner with pullStash set and conflicted
// files remaining.
function mountWithStash(pullStash: string, conflicted: ChangedFile[] = []) {
  useRepoStore().summary = summaryFixture({ pullStash })
  useChangesStore().files = changedFilesFixture({ conflicted })
  return mount(StashBanner, { props: { wt: '/repo' } })
}

describe('StashBanner', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.PopStash.mockReset().mockResolvedValue(undefined)
    Facade.DropStash.mockReset().mockResolvedValue(undefined)
    confirmRequire.mockReset()
  })

  it('FR-SYNC-09 does not render with no pull stash', () => {
    const wrapper = mountWithStash('')

    expect(wrapper.find('[data-test="stash-banner"]').exists()).toBe(false)
  })

  it('FR-SYNC-09 renders while a pull stash is set and nothing is conflicted', () => {
    const wrapper = mountWithStash('stash@{0}')

    expect(wrapper.get('[data-test="stash-banner"]').text()).toContain('stashed')
  })

  it('FR-SYNC-09 is hidden while a conflicted file remains', () => {
    const wrapper = mountWithStash('stash@{0}', [{ path: 'a.txt', status: 'UU' }])

    expect(wrapper.find('[data-test="stash-banner"]').exists()).toBe(false)
  })

  it('FR-SYNC-09 clicking Restore calls Facade.PopStash with the stash ref', async () => {
    const wrapper = mountWithStash('stash@{0}')
    await findButton(wrapper, 'Restore')!.trigger('click')

    expect(Facade.PopStash).toHaveBeenCalledWith('/repo', 'stash@{0}')
  })

  it('FR-SYNC-09, FR-UX-03, QA-SAFE-01 clicking Drop… confirms, naming the loss, before calling Facade.DropStash', async () => {
    const wrapper = mountWithStash('stash@{0}')
    await findButton(wrapper, 'Drop…')!.trigger('click')

    expect(confirmRequire).toHaveBeenCalledTimes(1)
    const options = confirmOptions()
    expect(options.message).toContain('lost')
    expect(Facade.DropStash).not.toHaveBeenCalled()

    options.accept()

    expect(Facade.DropStash).toHaveBeenCalledTimes(1)
    expect(Facade.DropStash).toHaveBeenCalledWith('/repo', 'stash@{0}')
  })
})
