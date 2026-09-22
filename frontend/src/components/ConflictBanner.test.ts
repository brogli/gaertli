import { mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockContinueOperation, mockAbortOperation, mockRequire } = vi.hoisted(() => ({
  mockContinueOperation: vi.fn(),
  mockAbortOperation: vi.fn(),
  mockRequire: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: {
    ContinueOperation: mockContinueOperation,
    AbortOperation: mockAbortOperation,
  },
}))

vi.mock('openvue/useconfirm', () => ({
  useConfirm: () => ({ require: mockRequire, close: vi.fn() }),
}))

import ConflictBanner from './ConflictBanner.vue'
import { useRepoStore } from '../stores/repo'
import { useChangesStore } from '../stores/changes'
import { changedFilesFixture } from '../testing/fixtures/changes'
import { summaryFixture } from '../testing/fixtures/summary'

function findButton(wrapper: VueWrapper, text: string) {
  return wrapper.findAll('button').find((b) => b.text() === text)
}

describe('ConflictBanner', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockContinueOperation.mockReset().mockResolvedValue(undefined)
    mockAbortOperation.mockReset().mockResolvedValue(undefined)
    mockRequire.mockReset()
  })

  it('FR-CONF-04 does not render while no operation is in progress', () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ operation: '' })

    const wrapper = mount(ConflictBanner, { props: { wt: '/repo' } })

    expect(wrapper.find('[data-test="conflict-banner"]').exists()).toBe(false)
  })

  it('FR-CONF-04 disables Continue while a conflicted file remains, enabling once cleared', async () => {
    const repo = useRepoStore()
    const changes = useChangesStore()
    repo.summary = summaryFixture({ operation: 'merge' })
    changes.files = changedFilesFixture({ conflicted: [{ path: 'a.txt', status: 'UU' }] })

    const wrapper = mount(ConflictBanner, { props: { wt: '/repo' } })
    expect(findButton(wrapper, 'Continue')!.attributes('disabled')).not.toBeUndefined()

    changes.files = changedFilesFixture({ conflicted: [] })
    await wrapper.vm.$nextTick()

    expect(findButton(wrapper, 'Continue')!.attributes('disabled')).toBeUndefined()
  })

  it('FR-CONF-04, FR-UX-03, QA-SAFE-01 clicking Abort confirms before calling Facade.AbortOperation', async () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ operation: 'merge' })

    const wrapper = mount(ConflictBanner, { props: { wt: '/repo' } })
    await findButton(wrapper, 'Abort…')!.trigger('click')

    expect(mockRequire).toHaveBeenCalledTimes(1)
    const options = mockRequire.mock.calls[0]![0] as { accept: () => void }
    expect(mockAbortOperation).not.toHaveBeenCalled()

    options.accept()

    expect(mockAbortOperation).toHaveBeenCalledWith('/repo')
  })

  it('FR-CONF-04, FR-UX-03 declining the confirm dialog does not call Facade.AbortOperation', async () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ operation: 'merge' })

    const wrapper = mount(ConflictBanner, { props: { wt: '/repo' } })
    await findButton(wrapper, 'Abort…')!.trigger('click')

    expect(mockAbortOperation).not.toHaveBeenCalled()
  })
})
