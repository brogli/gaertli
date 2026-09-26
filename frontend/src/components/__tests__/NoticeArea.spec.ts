import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, nextTick } from 'vue'
import NoticeArea from '../NoticeArea.vue'
import { mountInBody } from '@/testing/mountInBody'
import { findButton } from '@/testing/findButton'
import { errorNotice, useNotices, type AppNotice } from '@/lib/notice'
import { Code } from '../../../bindings/github.com/brogli/nebegit/internal/core'

// mountShow mounts a throwaway component whose setup shows message under
// kind 'test'.
function mountShow(message: AppNotice): void {
  mount(
    defineComponent({
      setup() {
        useNotices().show('test', message)
        return () => null
      },
    }),
  )
}

// showNotice mounts the notice area and shows message under kind.
async function showNotice(message: AppNotice) {
  const { body } = await mountInBody(NoticeArea)
  mountShow(message)
  await flushPromises()
  return body
}

const failure = errorNotice({
  code: Code.GitFailed,
  message: 'fatal: could not read from remote',
  command: 'git fetch origin',
  gitStderr: 'fatal: could not read from remote',
})

describe('NoticeArea', () => {
  it('FR-UX-01 an error notice shows top right in the standard error look, its headline as summary', async () => {
    const body = await showNotice(failure)

    expect(body.find('.p-toast').classes()).toContain('p-toast-top-right')
    const message = body.get('.p-toast-message')
    expect(message.classes()).toContain('p-toast-message-error')
    expect(message.find('.p-toast-message-icon').exists()).toBe(true)
    expect(message.get('.p-toast-summary').text()).toBe('fatal: could not read from remote')
  })

  it("FR-UX-01 an error notice shows git's command and stderr inside the message behind Show details", async () => {
    const body = await showNotice(failure)
    const message = body.find('.p-toast-message')
    expect(message.find('pre').exists()).toBe(false)

    await findButton(message, 'show details')!.trigger('click')

    expect(message.get('pre').text()).toContain('git fetch origin')
  })

  it('FR-UX-01 a success notice with a hash shows in the success look; its Copy button copies the hash', async () => {
    const writeText = vi.fn()
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    const body = await showNotice({
      severity: 'success',
      summary: 'Reset main',
      data: { copyText: 'deadbeef' },
    })

    expect(body.get('.p-toast-message').classes()).toContain('p-toast-message-success')
    const copy = body.get('[data-test="notice-copy"]')
    expect(copy.classes()).toContain('p-button')
    await copy.trigger('click')
    expect(writeText).toHaveBeenCalledWith('deadbeef')
  })

  it("FR-UX-15 the Toast's own close button closes the notice", async () => {
    const body = await showNotice(failure)

    await body.get('.p-toast-close-button').trigger('click')
    await flushPromises()

    expect(body.find('.p-toast-message').exists()).toBe(false)
  })

  describe('closing after 5 s', () => {
    afterEach(() => {
      vi.useRealTimers()
    })

    // showThenFakeTimers mounts the notice area under real timers, then
    // switches to fake timers before showing message: its close timeout,
    // set as the message mounts, is one the test can advance.
    async function showThenFakeTimers(message: AppNotice) {
      const { body } = await mountInBody(NoticeArea)
      vi.useFakeTimers()
      mountShow(message)
      await nextTick()
      return body
    }

    it.each<[string, AppNotice, boolean]>([
      [
        'a success notice without buttons is gone',
        { severity: 'success', summary: 'Reset main' },
        false,
      ],
      ['a warning notice without buttons is gone', { severity: 'warn', summary: 'Careful' }, false],
      ['an error notice is still shown', failure, true],
      [
        'a success notice with Copy hash is still shown',
        { severity: 'success', summary: 'Reset main', data: { copyText: 'deadbeef' } },
        true,
      ],
      [
        'a warning notice with an action button is still shown',
        {
          severity: 'warn',
          summary: 'Careful',
          data: { actions: [{ label: 'Undo', run: () => {} }] },
        },
        true,
      ],
    ])('FR-UX-15 %s after 5 s', async (_, message, stillShown) => {
      const body = await showThenFakeTimers(message)

      await vi.advanceTimersByTimeAsync(5000)

      expect(body.find('.p-toast-message').exists()).toBe(stillShown)
    })
  })
})
