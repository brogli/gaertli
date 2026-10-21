// Requirements: FR-SYNC-09, FR-UX-03, QA-SAFE-01
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({ Facade: { PopStash: vi.fn(), DropStash: vi.fn() } }))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

vi.mock('openvue/useconfirm', () => import('@/testing/confirm'))
import { confirmOptions, confirmRequire } from '@/testing/confirm'

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))

import StashBanner from '../StashBanner.vue'
import { useRepoStore } from '@/stores/repo'
import { useChangesStore } from '@/stores/changes'
import { changedFilesFixture } from '@/testing/fixtures/changes'
import { findButton } from '@/testing/findButton'
import { summaryFixture } from '@/testing/fixtures/summary'
import type { ChangedFile } from '../../../bindings/github.com/brogli/nebegit/internal/core'

// mountWithStash mounts StashBanner with waitingStash set and conflicted
// files remaining.
function mountWithStash(waitingStash: string, conflicted: ChangedFile[] = []) {
  useRepoStore().summary = summaryFixture({ waitingStash })
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

  it.each([
    { state: 'no waiting stash', stash: '', conflicted: [], shown: false },
    {
      state: 'a waiting stash and nothing conflicted',
      stash: 'stash@{0}',
      conflicted: [],
      shown: true,
    },
    {
      state: 'a waiting stash while a conflicted file remains',
      stash: 'stash@{0}',
      conflicted: [{ path: 'a.txt', status: 'UU' }],
      shown: false,
    },
  ])('with $state the banner shows: $shown', ({ stash, conflicted, shown }) => {
    const wrapper = mountWithStash(stash, conflicted)

    expect(wrapper.find('[data-test="stash-banner"]').exists()).toBe(shown)
    expect(wrapper.text().includes('Local changes from before a pull, merge or rebase')).toBe(shown)
  })

  it('clicking Restore calls Facade.PopStash with the stash ref', async () => {
    const wrapper = mountWithStash('stash@{0}')
    await findButton(wrapper, 'Restore')!.trigger('click')

    expect(Facade.PopStash).toHaveBeenCalledWith('/repo', 'stash@{0}')
  })

  it('clicking Drop… confirms, naming the loss, before calling Facade.DropStash', async () => {
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
