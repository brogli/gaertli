// Requirements: FR-ED-06, FR-UX-15
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'

vi.mock(
  '../../../bindings/github.com/brogli/nebegit/internal/app',
  () => import('@/testing/mainWindowApp'),
)
import { Facade, resetMainWindowApp } from '@/testing/mainWindowApp'

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))

import { useCoreCall } from '../useCoreCall'
import { useOpenInEditor } from '../useOpenInEditor'
import { editorNotFound, wailsError } from '@/testing/errors'
import { mountNotices } from '@/testing/mountNotices'

// mountHost mounts a component using useOpenInEditor and returns its
// actions and the error run left behind.
function mountHost() {
  let host!: ReturnType<typeof useOpenInEditor> & { error: ReturnType<typeof useCoreCall>['error'] }
  const Host = defineComponent({
    setup() {
      const { error, run } = useCoreCall()
      host = { ...useOpenInEditor('/repo', run), error }
      return () => null
    },
  })
  mount(Host)
  return host
}

const opens = [
  ['openEditor', Facade.OpenFile, (h: ReturnType<typeof mountHost>) => h.openEditor({ path: 'a' })],
  ['openFolder', Facade.OpenFolder, (h: ReturnType<typeof mountHost>) => h.openFolder()],
  [
    'openFolderAt',
    Facade.OpenFolderAt,
    (h: ReturnType<typeof mountHost>) => h.openFolderAt('/other'),
  ],
] as const

describe('useOpenInEditor', () => {
  beforeEach(() => {
    resetMainWindowApp()
  })

  it.each(opens)(
    '%s failing EDITOR_NOT_FOUND shows an error notice naming the command, no error left on the call',
    async (_name, call, open) => {
      call.mockRejectedValueOnce(editorNotFound('myed'))
      const notice = await mountNotices()
      const host = mountHost()

      open(host)
      await flushPromises()

      expect(notice.get('.p-toast-message').text()).toContain('myed')
      expect(host.error.value).toBeNull()
    },
  )

  it('any other failure stays on the call as its error, with no notice', async () => {
    Facade.OpenFile.mockRejectedValueOnce(wailsError('disk gone', 'GIT_FAILED'))
    const notice = await mountNotices()
    const host = mountHost()

    host.openEditor({ path: 'a' })
    await flushPromises()

    expect(host.error.value?.message).toBe('disk gone')
    expect(notice.find('.p-toast-message').exists()).toBe(false)
  })

  it("a second EDITOR_NOT_FOUND failure replaces the first one's notice", async () => {
    Facade.OpenFile.mockRejectedValueOnce(editorNotFound('alpha')).mockRejectedValueOnce(
      editorNotFound('beta'),
    )
    const notice = await mountNotices()
    const host = mountHost()

    host.openEditor({ path: 'a' })
    await flushPromises()
    host.openEditor({ path: 'a' })
    await flushPromises()

    expect(notice.findAll('.p-toast-message')).toHaveLength(1)
    expect(notice.text()).toContain('beta')
  })
})
