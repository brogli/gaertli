import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import DebugArea from './DebugArea.vue'
import NoticeArea from './NoticeArea.vue'
import { mountInBody } from '../testing/mountInBody'
import { findButton } from '../testing/findButton'

// mountWithNotices mounts NoticeArea (where a shown notice renders) and
// DebugArea separately, as real windows do: NoticeArea sits in App.vue,
// DebugArea inside MainView.vue.
async function mountWithNotices() {
  const { body } = await mountInBody(NoticeArea)
  const debug = mount(DebugArea)
  await flushPromises()
  return { debug, body }
}

describe('DebugArea', () => {
  it('FR-UX-19 has an error, a warning and a success button', async () => {
    const { debug } = await mountWithNotices()

    expect(findButton(debug, 'Error')).toBeTruthy()
    expect(findButton(debug, 'Warning')).toBeTruthy()
    expect(findButton(debug, 'Success')).toBeTruthy()
  })

  it('FR-UX-19 each button shows a sample notice of its own severity, through the normal notice path', async () => {
    const { debug, body } = await mountWithNotices()

    await findButton(debug, 'Error')!.trigger('click')
    await flushPromises()

    const message = body.get('.p-toast-message')
    expect(message.classes()).toContain('p-toast-message-error')
    expect(message.get('.p-toast-summary').text()).toBe('Sample error notice')
  })

  it('FR-UX-19 the three sample notices stack instead of replacing each other', async () => {
    const { debug, body } = await mountWithNotices()

    await findButton(debug, 'Error')!.trigger('click')
    await findButton(debug, 'Warning')!.trigger('click')
    await findButton(debug, 'Success')!.trigger('click')
    await flushPromises()

    const severities = body
      .findAll('.p-toast-message')
      .map((m) => m.classes().find((c) => c.startsWith('p-toast-message-')))
    expect(severities).toEqual([
      'p-toast-message-error',
      'p-toast-message-warn',
      'p-toast-message-success',
    ])
  })

  describe('closing after 5 s', () => {
    afterEach(() => {
      vi.useRealTimers()
    })

    it('FR-UX-19 5 s after showing all three samples, only the error remains', async () => {
      const { debug, body } = await mountWithNotices()
      vi.useFakeTimers()

      await findButton(debug, 'Error')!.trigger('click')
      await findButton(debug, 'Warning')!.trigger('click')
      await findButton(debug, 'Success')!.trigger('click')
      await vi.advanceTimersByTimeAsync(5000)

      const severities = body
        .findAll('.p-toast-message')
        .map((m) => m.classes().find((c) => c.startsWith('p-toast-message-')))
      expect(severities).toEqual(['p-toast-message-error'])
    })
  })
})
