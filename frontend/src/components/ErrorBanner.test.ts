import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ErrorBanner from './ErrorBanner.vue'

describe('ErrorBanner', () => {
  it('FR-ED-06 shows an "Open editor setting" button for EDITOR_NOT_FOUND, emitting open-settings on click', async () => {
    const wrapper = mount(ErrorBanner, {
      props: { error: { message: 'no editor configured', code: 'EDITOR_NOT_FOUND' } },
    })

    const button = wrapper.findAll('button').find((b) => b.text() === 'Open editor setting')
    expect(button).toBeTruthy()

    await button!.trigger('click')

    expect(wrapper.emitted('open-settings')).toHaveLength(1)
  })

  it('FR-ED-06 shows no "Open editor setting" button for a different error code', () => {
    const wrapper = mount(ErrorBanner, {
      props: { error: { message: 'not a git repository', code: 'NOT_A_REPO' } },
    })

    expect(wrapper.findAll('button').find((b) => b.text() === 'Open editor setting')).toBeUndefined()
  })
})
