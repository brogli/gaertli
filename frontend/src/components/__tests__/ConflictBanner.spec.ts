import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade, AppService } = vi.hoisted(() => ({
  Facade: {
    ContinueOperation: vi.fn(),
    AbortOperation: vi.fn(),
    SkipOperation: vi.fn(),
    Status: vi.fn(),
  },
  AppService: { OpenDiff: vi.fn() },
}))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade, AppService }))

vi.mock('openvue/useconfirm', () => import('@/testing/confirm'))
import { confirmOptions, confirmRequire } from '@/testing/confirm'

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))

import ConflictBanner from '../ConflictBanner.vue'
import { useRepoStore } from '@/stores/repo'
import { useChangesStore } from '@/stores/changes'
import { changedFilesFixture } from '@/testing/fixtures/changes'
import { findButton } from '@/testing/findButton'
import { summaryFixture } from '@/testing/fixtures/summary'
import { DiffKind } from '@/lib/diffTarget'
import { Operation } from '../../../bindings/github.com/brogli/nebegit/internal/core'

// mountDuring mounts ConflictBanner while operation is in progress.
function mountDuring(operation: Operation) {
  useRepoStore().summary = summaryFixture({ operation })
  return mount(ConflictBanner, { props: { wt: '/repo' } })
}

describe('ConflictBanner', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.ContinueOperation.mockReset().mockResolvedValue(undefined)
    Facade.AbortOperation.mockReset().mockResolvedValue(undefined)
    Facade.SkipOperation.mockReset().mockResolvedValue(undefined)
    Facade.Status.mockReset()
    AppService.OpenDiff.mockReset().mockResolvedValue(undefined)
    confirmRequire.mockReset()
  })

  it.each([
    { hidden: 'the banner while no operation is in progress', operation: Operation.$zero },
    {
      hidden: '"Resolve conflicts" once no file is conflicted',
      operation: Operation.MergeOperation,
    },
  ])('FR-CONF-04 shows no $hidden', ({ operation }) => {
    useChangesStore().files = changedFilesFixture({ conflicted: [] })
    const wrapper = mountDuring(operation)

    expect(wrapper.find('[data-test="conflict-banner"]').exists()).toBe(
      operation !== Operation.$zero,
    )
    expect(findButton(wrapper, 'Resolve conflicts')).toBeUndefined()
  })

  it('FR-CONF-04 disables Continue while a conflicted file remains, enabling once cleared', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture({ conflicted: [{ path: 'a.txt', status: 'UU' }] })

    const wrapper = mountDuring(Operation.MergeOperation)
    expect(findButton(wrapper, 'Continue')!.attributes('disabled')).not.toBeUndefined()

    changes.files = changedFilesFixture({ conflicted: [] })
    await wrapper.vm.$nextTick()

    expect(findButton(wrapper, 'Continue')!.attributes('disabled')).toBeUndefined()
  })

  it('FR-CONF-09 "Resolve conflicts" opens the diff window on the first conflicted file', async () => {
    const conflicted = [
      { path: 'a.txt', status: 'UU' },
      { path: 'b.txt', status: 'UU' },
    ]
    Facade.Status.mockResolvedValue(changedFilesFixture({ conflicted }))
    useChangesStore().files = changedFilesFixture({ conflicted })
    const wrapper = mountDuring(Operation.MergeOperation)

    await findButton(wrapper, 'Resolve conflicts')!.trigger('click')
    await flushPromises()

    expect(AppService.OpenDiff).toHaveBeenCalledExactlyOnceWith('/repo', {
      kind: DiffKind.WorkingDiff,
      conflicted: true,
      path: 'a.txt',
      oldPath: '',
    })
  })

  it('FR-CONF-04, FR-UX-03, QA-SAFE-01 clicking Abort confirms before calling Facade.AbortOperation', async () => {
    const wrapper = mountDuring(Operation.MergeOperation)
    await findButton(wrapper, 'Abort…')!.trigger('click')

    expect(confirmRequire).toHaveBeenCalledTimes(1)
    const options = confirmOptions()
    expect(Facade.AbortOperation).not.toHaveBeenCalled()

    options.accept()

    expect(Facade.AbortOperation).toHaveBeenCalledWith('/repo')
  })

  it.each([
    { operation: Operation.CherryPickOperation, skips: true },
    { operation: Operation.RevertOperation, skips: true },
    { operation: Operation.MergeOperation, skips: false },
  ])(
    'FR-CP-07 Skip is offered only while a cherry-pick or revert is stopped ($operation)',
    ({ operation, skips }) => {
      const wrapper = mountDuring(operation)

      expect(findButton(wrapper, 'Skip') !== undefined).toBe(skips)
    },
  )

  it('FR-CP-07 clicking Skip calls Facade.SkipOperation', async () => {
    const wrapper = mountDuring(Operation.CherryPickOperation)
    await findButton(wrapper, 'Skip')!.trigger('click')

    expect(Facade.SkipOperation).toHaveBeenCalledWith('/repo')
  })
})
