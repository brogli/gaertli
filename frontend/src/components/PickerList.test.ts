import { flushPromises, mount } from '@vue/test-utils'
import { h } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import PickerList from './PickerList.vue'
import type { PickerRow } from './pickerRow'

type Row = PickerRow & { label: string }

const rows: Row[] = [
  { key: 'h', label: 'Group', header: true },
  { key: 'a', label: 'alpha' },
  { key: 'b', label: 'beta' },
]

// mountList renders each row as a clickable span (wired to the slot's
// activate) plus, per row, a sibling button that calls action with the
// row's key; waits for the rows in view to render.
async function mountList(action = vi.fn(), listRows: Row[] = rows) {
  const wrapper = mount(PickerList<Row>, {
    props: { rows: listRows, placeholder: 'Filter' },
    slots: {
      row: ({ row, activate }: { row: Row; activate: () => void }) => [
        h('span', { class: 'main', onClick: activate }, row.label),
        h('button', { class: 'act', onClick: () => action(row.key) }),
      ],
    },
    attachTo: document.body,
  })
  await flushPromises()
  return wrapper
}

describe('PickerList', () => {
  it('FR-WT-13 a button in a row runs its own action and does not activate the row', async () => {
    const action = vi.fn()
    const wrapper = await mountList(action)

    await wrapper.findAll('.act')[2]!.trigger('click')

    expect(action).toHaveBeenCalledWith('b')
    expect(wrapper.emitted('activate')).toBeUndefined()
  })

  it('FR-WT-13 a click on a row activates it; a header row does nothing', async () => {
    const wrapper = await mountList()

    await wrapper.findAll('li')[0]!.get('.main').trigger('click')
    await wrapper.findAll('li')[2]!.get('.main').trigger('click')

    expect(wrapper.emitted('activate')).toEqual([[rows[2]]])
  })

  it('FR-WT-13 the filter has focus; arrows move the active row past headers and Enter activates it', async () => {
    const wrapper = await mountList()
    const filter = wrapper.get('input')
    expect(document.activeElement).toBe(filter.element)

    await filter.trigger('keydown', { key: 'ArrowDown' })
    await filter.trigger('keydown', { key: 'ArrowDown' })
    await filter.trigger('keydown', { key: 'ArrowUp' })
    await filter.trigger('keydown', { key: 'ArrowUp' })
    await filter.trigger('keydown', { key: 'Enter' })

    expect(wrapper.emitted('activate')).toEqual([[rows[1]]])
    expect(wrapper.get('li.active').text()).toBe('alpha')
  })

  it('FR-BR-09 a long list renders only the rows in view', async () => {
    const many = Array.from({ length: 2000 }, (_, i) => ({ key: `r${i}`, label: `ref ${i}` }))
    const wrapper = await mountList(vi.fn(), many)

    const rendered = wrapper.findAll('li')
    expect(rendered.length).toBeGreaterThan(10)
    expect(rendered.length).toBeLessThan(100)
    expect(rendered[0]!.text()).toBe('ref 0')
  })

  it('FR-WT-13 typing emits the query and makes the first row active again', async () => {
    const wrapper = await mountList()
    const filter = wrapper.get('input')
    await filter.trigger('keydown', { key: 'ArrowDown' })

    await filter.setValue('be')
    await wrapper.setProps({ rows: [rows[0]!, rows[2]!] })
    await filter.trigger('keydown', { key: 'Enter' })

    expect(wrapper.emitted('update:query')).toEqual([['be']])
    expect(wrapper.emitted('activate')).toEqual([[rows[2]]])
  })
})
