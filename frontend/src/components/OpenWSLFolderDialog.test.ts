import { flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { AppService } = vi.hoisted(() => ({
  AppService: { ListWSLDistros: vi.fn(), OpenPath: vi.fn() },
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({ AppService }))
vi.mock('@wailsio/runtime', () => import('../testing/runtime'))

import OpenWSLFolderDialog from './OpenWSLFolderDialog.vue'
import { mountInBody } from '../testing/mountInBody'

describe('OpenWSLFolderDialog', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    AppService.ListWSLDistros.mockReset().mockResolvedValue([{ name: 'Ubuntu', version: 2 }])
    AppService.OpenPath.mockReset().mockResolvedValue(undefined)
  })

  it.each(['/home/u/r', 'home/u/r'])(
    'FR-REPO-07 a typed folder %s opens as \\\\wsl.localhost\\Ubuntu\\home\\u\\r',
    async (typed) => {
      const { body } = await mountInBody(OpenWSLFolderDialog)
      await body.get('[data-test="wsl-distro"]').trigger('click')
      await body.get('input[placeholder="Folder inside the distro"]').setValue(typed)
      await body.get('[data-test="wsl-open"]').trigger('click')
      await flushPromises()

      expect(AppService.OpenPath).toHaveBeenCalledWith('\\\\wsl.localhost\\Ubuntu\\home\\u\\r')
    },
  )
})
