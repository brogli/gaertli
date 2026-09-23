import { mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockPopStash, mockDropStash, mockRequire } = vi.hoisted(() => ({
  mockPopStash: vi.fn(),
  mockDropStash: vi.fn(),
  mockRequire: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { PopStash: mockPopStash, DropStash: mockDropStash },
}))

vi.mock('openvue/useconfirm', () => ({
  useConfirm: () => ({ require: mockRequire, close: vi.fn() }),
}))

import StashNotice from './StashNotice.vue'
import { useRepoStore } from '../stores/repo'
import { useChangesStore } from '../stores/changes'
import { changedFilesFixture } from '../testing/fixtures/changes'
import { summaryFixture } from '../testing/fixtures/summary'

function findButton(wrapper: VueWrapper, text: string) {
  return wrapper.findAll('button').find((b) => b.text() === text)
}

describe('StashNotice', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockPopStash.mockReset().mockResolvedValue(undefined)
    mockDropStash.mockReset().mockResolvedValue(undefined)
    mockRequire.mockReset()
  })

  it('FR-SYNC-09 does not render with no pull stash', () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ pullStash: '' })

    const wrapper = mount(StashNotice, { props: { wt: '/repo' } })

    expect(wrapper.find('[data-test="stash-notice"]').exists()).toBe(false)
  })

  it('FR-SYNC-09 renders while a pull stash is set and nothing is conflicted', () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ pullStash: 'stash@{0}' })
    const changes = useChangesStore()
    changes.files = changedFilesFixture()

    const wrapper = mount(StashNotice, { props: { wt: '/repo' } })

    expect(wrapper.get('[data-test="stash-notice"]').text()).toContain('stashed')
  })

  it('FR-SYNC-09 is hidden while a conflicted file remains', () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ pullStash: 'stash@{0}' })
    const changes = useChangesStore()
    changes.files = changedFilesFixture({ conflicted: [{ path: 'a.txt', status: 'UU' }] })

    const wrapper = mount(StashNotice, { props: { wt: '/repo' } })

    expect(wrapper.find('[data-test="stash-notice"]').exists()).toBe(false)
  })

  it('FR-SYNC-09 clicking Restore calls Facade.PopStash with the stash ref', async () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ pullStash: 'stash@{0}' })
    const changes = useChangesStore()
    changes.files = changedFilesFixture()

    const wrapper = mount(StashNotice, { props: { wt: '/repo' } })
    await findButton(wrapper, 'Restore')!.trigger('click')

    expect(mockPopStash).toHaveBeenCalledWith('/repo', 'stash@{0}')
  })

  it('FR-SYNC-09, FR-UX-03, QA-SAFE-01 clicking Drop… confirms, naming the loss, before calling Facade.DropStash', async () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ pullStash: 'stash@{0}' })
    const changes = useChangesStore()
    changes.files = changedFilesFixture()

    const wrapper = mount(StashNotice, { props: { wt: '/repo' } })
    await findButton(wrapper, 'Drop…')!.trigger('click')

    expect(mockRequire).toHaveBeenCalledTimes(1)
    const options = mockRequire.mock.calls[0]![0] as { message: string; accept: () => void }
    expect(options.message).toContain('lost')
    expect(mockDropStash).not.toHaveBeenCalled()

    options.accept()

    expect(mockDropStash).toHaveBeenCalledTimes(1)
    expect(mockDropStash).toHaveBeenCalledWith('/repo', 'stash@{0}')
  })
})
