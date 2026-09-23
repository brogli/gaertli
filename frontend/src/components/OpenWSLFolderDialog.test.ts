import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockListWSLDistros, mockOpenPath } = vi.hoisted(() => ({
  mockListWSLDistros: vi.fn(),
  mockOpenPath: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  AppService: { ListWSLDistros: mockListWSLDistros, OpenPath: mockOpenPath },
}))

import OpenWSLFolderDialog from './OpenWSLFolderDialog.vue'
import { mountInBody } from '../testing/mountInBody'

describe('OpenWSLFolderDialog', () => {
  beforeEach(() => {
    mockListWSLDistros
      .mockReset()
      .mockResolvedValue([{ name: 'Ubuntu', version: 2, default: true }])
    mockOpenPath.mockReset().mockResolvedValue(undefined)
  })

  it.each(['/home/u/r', 'home/u/r'])(
    'FR-REPO-07 a typed folder %s opens as \\\\wsl.localhost\\Ubuntu\\home\\u\\r',
    async (typed) => {
      const { wrapper, body } = await mountInBody(OpenWSLFolderDialog)
      await body.get('[data-test="wsl-distro"]').trigger('click')
      await body.get('input[placeholder="Folder inside the distro"]').setValue(typed)
      await body.get('[data-test="wsl-open"]').trigger('click')
      await flushPromises()

      expect(mockOpenPath).toHaveBeenCalledWith('\\\\wsl.localhost\\Ubuntu\\home\\u\\r')
      wrapper.unmount()
    },
  )
})
