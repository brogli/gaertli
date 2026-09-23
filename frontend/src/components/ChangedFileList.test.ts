import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ChangedFileList from './ChangedFileList.vue'

const files = [
  { path: 'a.txt', status: 'M' },
  { path: 'b.txt', oldPath: 'a2.txt', status: 'R' },
]

describe('ChangedFileList', () => {
  it('FR-CMP-02 clicking a file row emits select with that file', async () => {
    const wrapper = mount(ChangedFileList, { props: { files } })

    await wrapper.findAll('li')[1]!.trigger('click')

    expect(wrapper.emitted('select')?.[0]).toEqual([files[1]])
  })

  it('FR-CMP-02 marks the selected file with a selected class', () => {
    const wrapper = mount(ChangedFileList, { props: { files, selected: files[1]! } })

    const rows = wrapper.findAll('li')
    expect(rows[0]!.classes()).not.toContain('selected')
    expect(rows[1]!.classes()).toContain('selected')
  })

  it('FR-CMP-02 double-clicking a file row emits open with that file', async () => {
    const wrapper = mount(ChangedFileList, { props: { files } })

    await wrapper.findAll('li')[1]!.trigger('dblclick')

    expect(wrapper.emitted('open')?.[0]).toEqual([files[1]])
  })

  it('FR-ED-02 clicking a file row\'s "Open in editor" control emits open-editor with that file, not select', async () => {
    const wrapper = mount(ChangedFileList, { props: { files } })

    await wrapper.findAll('[data-test="open-editor"]')[1]!.trigger('click')

    expect(wrapper.emitted('open-editor')?.[0]).toEqual([files[1]])
    expect(wrapper.emitted('select')).toBeUndefined()
  })

  it('FR-CHG-02 renders no action button without actionLabel', () => {
    const wrapper = mount(ChangedFileList, { props: { files } })

    expect(wrapper.find('[data-test="action"]').exists()).toBe(false)
  })

  it('FR-CHG-02 clicking a row\'s action button emits action with that file, not select', async () => {
    const wrapper = mount(ChangedFileList, { props: { files, actionLabel: 'Stage' } })

    const button = wrapper.findAll('[data-test="action"]')[1]!
    expect(button.attributes('aria-label')).toBe('Stage')
    await button.trigger('click')

    expect(wrapper.emitted('action')?.[0]).toEqual([files[1]])
    expect(wrapper.emitted('select')).toBeUndefined()
  })

  it('FR-CHG-10 renders no second action button without secondActionLabel', () => {
    const wrapper = mount(ChangedFileList, { props: { files, actionLabel: 'Stage' } })

    expect(wrapper.find('[data-test="second-action"]').exists()).toBe(false)
  })

  it('FR-CHG-10 clicking a row\'s second action button emits second-action with that file, not select or action', async () => {
    const wrapper = mount(ChangedFileList, {
      props: { files, actionLabel: 'Stage', secondActionLabel: 'Discard' },
    })

    const button = wrapper.findAll('[data-test="second-action"]')[1]!
    expect(button.attributes('aria-label')).toBe('Discard')
    await button.trigger('click')

    expect(wrapper.emitted('second-action')?.[0]).toEqual([files[1]])
    expect(wrapper.emitted('select')).toBeUndefined()
    expect(wrapper.emitted('action')).toBeUndefined()
  })

  it('FR-UX-09 every action button has a title and aria-label naming it, and the name\'s title is the full path', () => {
    const wrapper = mount(ChangedFileList, {
      props: { files, actionLabel: 'Stage', secondActionLabel: 'Discard' },
    })

    const row = wrapper.findAll('li')[1]!
    for (const [selector, label] of [
      ['[data-test="open-editor"]', 'Open in editor'],
      ['[data-test="action"]', 'Stage'],
      ['[data-test="second-action"]', 'Discard'],
    ] as const) {
      const button = row.get(selector)
      expect(button.attributes('title')).toBe(label)
      expect(button.attributes('aria-label')).toBe(label)
    }
    expect(row.get('.path').attributes('title')).toBe('b.txt')
  })
})
