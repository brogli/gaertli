// Requirements: FR-UX-17
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock(
  '../../../bindings/github.com/brogli/gaertli/internal/app',
  () => import('@/testing/mainWindowApp'),
)
vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))
import { resetMainWindowApp } from '@/testing/mainWindowApp'

import CreateBranchDialog from '../CreateBranchDialog.vue'
import CreateWorktreeDialog from '../CreateWorktreeDialog.vue'
import PickDialog from '../PickDialog.vue'
import RenameBranchDialog from '../RenameBranchDialog.vue'
import ResetDialog from '../ResetDialog.vue'
import SetUpstreamDialog from '../SetUpstreamDialog.vue'
import SquashDialog from '../SquashDialog.vue'
import { fieldLabels, footerButtons } from '@/testing/dialog'
import { mountInBody } from '@/testing/mountInBody'

const dialogs = [
  {
    name: 'CreateBranchDialog',
    mount: () =>
      mountInBody(CreateBranchDialog, {
        props: { wt: '/repo', start: { ref: 'refs/heads/main' } },
      }),
    labels: ['Branch name', 'Start point'],
    buttons: ['Cancel', 'Create'],
  },
  {
    name: 'RenameBranchDialog',
    mount: () => mountInBody(RenameBranchDialog, { props: { wt: '/repo', name: 'feature' } }),
    labels: ['Branch name'],
    buttons: ['Cancel', 'Rename'],
  },
  {
    name: 'CreateWorktreeDialog',
    mount: () => mountInBody(CreateWorktreeDialog, { props: { repoPath: '/repo' } }),
    labels: ['Branch', 'Folder name', 'Location'],
    buttons: ['Cancel', 'Create'],
  },
  {
    name: 'SquashDialog',
    mount: () =>
      mountInBody(SquashDialog, {
        props: {
          wt: '/repo',
          shas: ['sha-a', 'sha-b'],
          check: { pushed: false, message: 'A\n\nB' },
          fixup: false,
        },
      }),
    labels: ['Message of the new commit'],
    buttons: ['Cancel', 'Squash'],
  },
  {
    name: 'PickDialog',
    mount: () =>
      mountInBody(PickDialog, { props: { wt: '/repo', verb: 'cherry-pick', shas: ['a1'] } }),
    buttons: ['Cancel', 'Confirm'],
  },
  {
    name: 'ResetDialog',
    mount: () =>
      mountInBody(ResetDialog, {
        props: { wt: '/repo', branch: 'main', sha: 'abc123', subject: 'Fix the bug' },
      }),
    buttons: ['Cancel', 'Reset'],
  },
  {
    name: 'SetUpstreamDialog',
    mount: () =>
      mountInBody(SetUpstreamDialog, {
        props: { wt: '/repo', branch: 'feature', remote: 'origin' },
      }),
    buttons: ['Cancel', 'Push and set upstream'],
  },
]

describe('dialog layout', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    resetMainWindowApp()
  })

  it.each(dialogs)(
    '$name: Cancel and the action sit together at the bottom; Cancel closes',
    async ({ mount, buttons }) => {
      const { wrapper, body } = await mount()

      expect(footerButtons(body)).toEqual(buttons)
      await body.get('[data-test="dialog-cancel"]').trigger('click')
      expect(wrapper.emitted('close')).toHaveLength(1)
    },
  )

  it.each(dialogs.filter((d) => d.labels))(
    '$name: each field has its own label',
    async ({ mount, labels }) => {
      const { body } = await mount()

      expect(fieldLabels(body)).toEqual(labels)
    },
  )
})
