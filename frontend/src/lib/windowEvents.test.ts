import { flushPromises } from '@vue/test-utils'
import { expect, it, vi } from 'vitest'

vi.mock('@wailsio/runtime', () => import('../testing/runtime'))
import { emit } from '../testing/runtime'
import { onWindowEvent } from './windowEvents'

it('ADR-9 a window handles only the events whose sender is this window', async () => {
  const handler = vi.fn()
  onWindowEvent('command', handler)

  emit('command', { sender: 'window-1', data: 'mine' })
  emit('command', { sender: 'window-2', data: 'theirs' })
  emit('command', { sender: undefined, data: 'nobody' })
  await flushPromises()

  expect(handler.mock.calls.map(([ev]) => ev.data)).toEqual(['mine'])
})
