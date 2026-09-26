import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ClearableInput from '../ClearableInput.vue'

describe('ClearableInput', () => {
  it('FR-LOG-08 shows no clear button while empty', () => {
    const wrapper = mount(ClearableInput, {
      props: { modelValue: '', placeholder: 'Path', testId: 'clear-path' },
    })

    expect(wrapper.find('[data-test="clear-path"]').exists()).toBe(false)
  })

  it('FR-LOG-08 clicking the clear button empties the model and emits clear', async () => {
    const wrapper = mount(ClearableInput, {
      props: { modelValue: 'src/', placeholder: 'Path', testId: 'clear-path' },
    })

    await wrapper.get('[data-test="clear-path"]').trigger('click')

    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([''])
    expect(wrapper.emitted('clear')).toHaveLength(1)
  })

  it('FR-LOG-08 typing emits update:modelValue for the caller to debounce', async () => {
    const wrapper = mount(ClearableInput, {
      props: { modelValue: '', placeholder: 'Path', testId: 'clear-path' },
    })

    await wrapper.find('input').setValue('docs')

    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual(['docs'])
  })
})
