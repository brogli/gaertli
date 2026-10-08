import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ChangedFileList from '../ChangedFileList.vue'

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

  it('FR-DIFF-07 Enter on a focused row emits open with that file; Enter on its button does not', async () => {
    const wrapper = mount(ChangedFileList, { props: { files } })
    const row = wrapper.findAll('li')[1]!

    expect(row.attributes('tabindex')).toBe('0')
    await row.trigger('keydown', { key: 'Enter' })
    await row.get('[data-test="open-editor"]').trigger('keydown', { key: 'Enter' })

    expect(wrapper.emitted('open')).toEqual([[files[1]]])
  })

  it('FR-ED-02 clicking a file row\'s "Open in editor" control emits open-editor with that file, not select', async () => {
    const wrapper = mount(ChangedFileList, { props: { files } })

    await wrapper.findAll('[data-test="open-editor"]')[1]!.trigger('click')

    expect(wrapper.emitted('open-editor')?.[0]).toEqual([files[1]])
    expect(wrapper.emitted('select')).toBeUndefined()
  })

  it('FR-CHG-02 renders no action button without an action kind', () => {
    const wrapper = mount(ChangedFileList, { props: { files } })

    expect(wrapper.find('[data-test="action"]').exists()).toBe(false)
  })

  it("FR-CHG-02 clicking a row's action button emits action with that file, not select", async () => {
    const wrapper = mount(ChangedFileList, { props: { files, action: 'stage' } })

    const button = wrapper.findAll('[data-test="action"]')[1]!
    await button.trigger('click')

    expect(wrapper.emitted('action')?.[0]).toEqual([files[1]])
    expect(wrapper.emitted('select')).toBeUndefined()
  })

  it('FR-CHG-10 renders no second action button without a second action kind', () => {
    const wrapper = mount(ChangedFileList, { props: { files, action: 'stage' } })

    expect(wrapper.find('[data-test="second-action"]').exists()).toBe(false)
  })

  it("FR-CHG-10 clicking a row's second action button emits second-action with that file, not select or action", async () => {
    const wrapper = mount(ChangedFileList, {
      props: { files, action: 'stage', secondAction: () => 'discard' },
    })

    const button = wrapper.findAll('[data-test="second-action"]')[1]!
    await button.trigger('click')

    expect(wrapper.emitted('second-action')?.[0]).toEqual([files[1]])
    expect(wrapper.emitted('select')).toBeUndefined()
    expect(wrapper.emitted('action')).toBeUndefined()
  })

  it("FR-UX-09 every action button has a title and aria-label naming it, and the name's title is the full path", () => {
    const wrapper = mount(ChangedFileList, {
      props: { files, action: 'stage', secondAction: () => 'discard' },
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

  it("FR-CHG-02 the action button is the row's first element, before the status icon; open-in-editor and the second action stay at its end; a row with no action kind starts with the status icon", () => {
    const withActions = mount(ChangedFileList, {
      props: { files, action: 'unstage', secondAction: () => 'discard' },
    })
    const plain = mount(ChangedFileList, { props: { files } })

    const row = withActions.findAll('li')[1]!
    const children = [...row.element.children].map(
      (el) => el.getAttribute('data-test') || el.className,
    )
    expect(children[0]).toBe('action')
    expect(children[1]).toContain('status')
    expect(row.element.lastElementChild!.classList).toContain('actions')
    expect(
      [...row.get('.actions').element.children].map((el) => el.getAttribute('data-test')),
    ).toEqual(['open-editor', 'second-action'])
    expect(plain.findAll('li')[1]!.element.children[0]!.className).toContain('status')
  })

  it('FR-UX-11 the action button icon and label come from the action kind, not the caller’s own text', () => {
    const stage = mount(ChangedFileList, { props: { files, action: 'stage' } })
    const markResolved = mount(ChangedFileList, { props: { files, action: 'markResolved' } })

    const stageButton = stage.findAll('[data-test="action"]')[0]!
    const resolveButton = markResolved.findAll('[data-test="action"]')[0]!
    expect(stageButton.get('.p-button-icon').classes()).toContain('oi-plus')
    expect(stageButton.attributes('aria-label')).toBe('Stage')
    expect(resolveButton.get('.p-button-icon').classes()).toContain('oi-check')
    expect(resolveButton.attributes('aria-label')).toBe('Mark resolved')
  })

  it('FR-UX-13 each row shows its status as an icon with the label as its tooltip, not the status letter', () => {
    const wrapper = mount(ChangedFileList, {
      props: { files: [{ path: 'a.txt', status: 'M' }, files[1]!] },
    })

    const statuses = wrapper.findAll('.status')
    expect(statuses.every((s) => s.find('i').exists())).toBe(true)
    expect(statuses.map((s) => s.attributes('title'))).toEqual(['Modified', 'Renamed'])
    expect(statuses[0]!.text()).toBe('')
  })
})
