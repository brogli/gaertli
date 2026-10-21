// Requirements: FR-WT-13, FR-WT-15
import { h } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import type { MenuItem } from 'openvue/menuitem'
import PickerList from '../PickerList.vue'
import { mountInBody } from '@/testing/mountInBody'
import { menuLabels } from '@/testing/contextMenu'
import type { PickerRow } from '../pickerRow'

type Row = PickerRow & { label: string }

const rows: Row[] = [
  { key: 'h', label: 'Group', header: true },
  { key: 'a', label: 'alpha' },
  { key: 'b', label: 'beta' },
]

// menuFor gives each row a menu holding one item labelled after its key.
function menuFor(row: Row): MenuItem[] {
  return [{ label: `${row.key}-item` }]
}

// mountList renders each row's label, plus a trailing button that calls
// action with the row's key; waits for the rows in view to render.
async function mountList(
  action = vi.fn(),
  listRows: Row[] = rows,
  props: { menu?: (row: Row) => MenuItem[]; menuButton?: boolean } = {},
) {
  const { wrapper, body } = await mountInBody(PickerList<Row>, {
    props: { rows: listRows, placeholder: 'Filter', ...props },
    slots: {
      row: ({ row }: { row: Row }) => h('span', { class: 'main' }, row.label),
      trail: ({ row }: { row: Row }) =>
        h('button', { class: 'act', onClick: () => action(row.key) }),
    },
  })
  return { wrapper, body }
}

describe('PickerList', () => {
  it('a button in a row runs its own action and does not activate the row', async () => {
    const action = vi.fn()
    const { wrapper } = await mountList(action)

    await wrapper.findAll('.act')[2]!.trigger('click')

    expect(action).toHaveBeenCalledWith('b')
    expect(wrapper.emitted('activate')).toBeUndefined()
  })

  it('a click on a row activates it; a header row does nothing', async () => {
    const { wrapper } = await mountList()

    await wrapper.findAll('li')[0]!.get('.main').trigger('click')
    await wrapper.findAll('li')[2]!.get('.main').trigger('click')

    expect(wrapper.emitted('activate')).toEqual([[rows[2]]])
  })

  it('the filter has focus; arrows move the active row past headers and Enter activates it', async () => {
    const { wrapper } = await mountList()
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

  it('each non-header row gets an "Actions" button opening a menu; the header gets none', async () => {
    const { wrapper } = await mountList(vi.fn(), rows, { menu: menuFor, menuButton: true })

    const items = wrapper.findAll('li')
    expect(items[0]!.find('[data-test="row-actions"]').exists()).toBe(false)
    for (const item of items.slice(1)) {
      const button = item.get('[data-test="row-actions"]')
      expect(button.attributes('title')).toBe('Actions')
      expect(button.attributes('aria-label')).toBe('Actions')
      expect(button.attributes('aria-haspopup')).toBe('menu')
    }
  })

  it('clicking a row\'s "…" opens its menu, makes the row active, and does not activate it', async () => {
    const { wrapper, body } = await mountList(vi.fn(), rows, { menu: menuFor, menuButton: true })

    await wrapper.findAll('li')[2]!.get('[data-test="row-actions"]').trigger('click')

    expect(menuLabels(body)).toEqual(['b-item'])
    expect(wrapper.get('li.active').text()).toContain('beta')
    expect(wrapper.emitted('activate')).toBeUndefined()
  })

  it('a right-click, then Shift+F10, open each active row\'s menu; no "…" buttons; a header\'s right-click opens none', async () => {
    const { wrapper, body } = await mountList(vi.fn(), rows, { menu: menuFor })

    expect(wrapper.find('[data-test="row-actions"]').exists()).toBe(false)

    await wrapper.findAll('li')[0]!.trigger('contextmenu')
    expect(menuLabels(body)).toEqual([])

    await wrapper.findAll('li')[2]!.get('.main').trigger('contextmenu')
    expect(menuLabels(body)).toEqual(['b-item'])

    const filter = wrapper.get('input')
    await filter.trigger('keydown', { key: 'ArrowUp' })
    await filter.trigger('keydown', { key: 'F10', shiftKey: true })
    expect(menuLabels(body)).toEqual(['a-item'])
  })

  it('a long list renders only the rows in view', async () => {
    const many = Array.from({ length: 2000 }, (_, i) => ({ key: `r${i}`, label: `ref ${i}` }))
    const { wrapper } = await mountList(vi.fn(), many)

    const rendered = wrapper.findAll('li')
    expect(rendered.length).toBeGreaterThan(10)
    expect(rendered.length).toBeLessThan(100)
    expect(rendered[0]!.text()).toBe('ref 0')
  })

  it('typing emits the query and makes the first row active again', async () => {
    const { wrapper } = await mountList()
    const filter = wrapper.get('input')
    await filter.trigger('keydown', { key: 'ArrowDown' })

    await filter.setValue('be')
    await wrapper.setProps({ rows: [rows[0]!, rows[2]!] })
    await filter.trigger('keydown', { key: 'Enter' })

    expect(wrapper.emitted('update:query')).toEqual([['be']])
    expect(wrapper.emitted('activate')).toEqual([[rows[2]]])
  })
})
