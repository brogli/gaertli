import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({
  Facade: { ContinueOperation: vi.fn(), AbortOperation: vi.fn(), SkipOperation: vi.fn() },
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

vi.mock('openvue/useconfirm', () => import('../testing/confirm'))
import { confirmOptions, confirmRequire } from '../testing/confirm'

import ConflictBanner from './ConflictBanner.vue'
import { useRepoStore } from '../stores/repo'
import { useChangesStore } from '../stores/changes'
import { changedFilesFixture } from '../testing/fixtures/changes'
import { findButton } from '../testing/findButton'
import { summaryFixture } from '../testing/fixtures/summary'
import { Operation } from '../../bindings/github.com/brogli/nebegit/internal/core'

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
    confirmRequire.mockReset()
  })

  it('FR-CONF-04 does not render while no operation is in progress', () => {
    const wrapper = mountDuring(Operation.$zero)

    expect(wrapper.find('[data-test="conflict-banner"]').exists()).toBe(false)
  })

  it('FR-CONF-04 disables Continue while a conflicted file remains, enabling once cleared', async () => {
    const repo = useRepoStore()
    const changes = useChangesStore()
    repo.summary = summaryFixture({ operation: Operation.MergeOperation })
    changes.files = changedFilesFixture({ conflicted: [{ path: 'a.txt', status: 'UU' }] })

    const wrapper = mount(ConflictBanner, { props: { wt: '/repo' } })
    expect(findButton(wrapper, 'Continue')!.attributes('disabled')).not.toBeUndefined()

    changes.files = changedFilesFixture({ conflicted: [] })
    await wrapper.vm.$nextTick()

    expect(findButton(wrapper, 'Continue')!.attributes('disabled')).toBeUndefined()
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

  it('FR-CONF-04, FR-UX-03 declining the confirm dialog does not call Facade.AbortOperation', async () => {
    const wrapper = mountDuring(Operation.MergeOperation)
    await findButton(wrapper, 'Abort…')!.trigger('click')

    expect(Facade.AbortOperation).not.toHaveBeenCalled()
  })

  it.each([Operation.CherryPickOperation, Operation.RevertOperation])(
    'FR-CP-07 renders a Skip button while a %s is stopped',
    (operation) => {
      const wrapper = mountDuring(operation)

      expect(findButton(wrapper, 'Skip')).toBeDefined()
    },
  )

  it('FR-CP-07 does not render a Skip button for a merge', () => {
    const wrapper = mountDuring(Operation.MergeOperation)

    expect(findButton(wrapper, 'Skip')).toBeUndefined()
  })

  it('FR-CP-07 clicking Skip calls Facade.SkipOperation', async () => {
    const wrapper = mountDuring(Operation.CherryPickOperation)
    await findButton(wrapper, 'Skip')!.trigger('click')

    expect(Facade.SkipOperation).toHaveBeenCalledWith('/repo')
  })
})
