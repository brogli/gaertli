<script setup lang="ts" generic="Row extends PickerRow">
import { computed, onMounted, ref, useTemplateRef, watch } from 'vue'
import { useVirtualizer } from '@tanstack/vue-virtual'
import InputText from 'openvue/inputtext'
import Button from 'openvue/button'
import ContextMenu from 'openvue/contextmenu'
import type { MenuItem } from 'openvue/menuitem'
import { useI18n } from '@/i18n'
import { iconClass } from '@/lib/icons'
import { ICON_BUTTON } from '@/lib/layout'
import type { PickerRow } from './pickerRow'

// PickerList is the searchable list of the toolbar pickers: a filter field
// that has focus, the arrow keys move the active row, Enter or a click
// activates a row. It owns the row menu too: a right-click on a row, or
// Shift+F10 or the context-menu key in the filter for the active row, opens
// it; a picker asking for a "…" button also gets one on each row whose menu
// holds items. A header row only labels a group, has no menu. The parent
// filters rows by query. Only the rows in view are rendered. Each row is a
// clickable part holding the row slot, with rowAttrs(row) as its
// attributes, between the lead and trail slots; a button in those slots is
// a sibling of the clickable part, so its click never activates the row. A
// right-button press stops at its row: OpenVue's Popover counts any
// mousedown in it as a click inside, and a right-click sends no click to
// reset that, so the next click outside would not close the list.
const props = defineProps<{
  rows: Row[]
  placeholder: string
  rowAttrs?: (row: Row) => Record<string, string>
  menu?: (row: Row) => MenuItem[]
  menuButton?: boolean
}>()
const query = defineModel<string>('query', { default: '' })
const emit = defineEmits<{ activate: [row: Row] }>()
defineSlots<{
  lead?(props: { row: Row }): unknown
  row(props: { row: Row }): unknown
  trail?(props: { row: Row }): unknown
}>()
const { t } = useI18n()

// Every row is one line of this height, so the virtualizer never measures.
const ROW_HEIGHT = 28

const scroller = useTemplateRef<HTMLElement>('scroller')
const virtualizer = useVirtualizer(
  computed(() => ({
    count: props.rows.length,
    getScrollElement: () => scroller.value,
    estimateSize: () => ROW_HEIGHT,
    overscan: 10,
  })),
)

function firstKey(): string | undefined {
  return props.rows.find((r) => !r.header)?.key
}

// A new query starts over at the first row; other changes to the rows keep
// the active one while it is still listed.
const activeKey = ref(firstKey())
watch(query, () => (activeKey.value = firstKey()))
watch(
  () => props.rows,
  (rows) => {
    if (!rows.some((r) => !r.header && r.key === activeKey.value)) activeKey.value = firstKey()
  },
)

function move(step: number): void {
  const rows = props.rows.filter((r) => !r.header)
  const i = rows.findIndex((r) => r.key === activeKey.value) + step
  activeKey.value = rows[Math.min(Math.max(i, 0), rows.length - 1)]?.key
  virtualizer.value.scrollToIndex(props.rows.findIndex((r) => r.key === activeKey.value))
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'ArrowDown') move(1)
  else if (event.key === 'ArrowUp') move(-1)
  else if (event.key === 'Enter') activateActive()
  else if (event.key === 'ContextMenu' || (event.shiftKey && event.key === 'F10')) openActiveMenu()
  else return
  event.preventDefault()
}

function activeRow(): Row | undefined {
  return props.rows.find((r) => r.key === activeKey.value)
}

function activateActive(): void {
  const row = activeRow()
  if (row) emit('activate', row)
}

function activate(row: Row): void {
  if (!row.header) emit('activate', row)
}

// rowMenu is row's menu items, empty for a header row or without a menu
// prop.
function rowMenu(row: Row): MenuItem[] {
  return row.header ? [] : (props.menu?.(row) ?? [])
}

const contextMenu = useTemplateRef<InstanceType<typeof ContextMenu>>('contextMenu')
const menuModel = ref<MenuItem[]>([])

// openMenu makes row the active row and opens its menu at event, unless it
// holds no items.
function openMenu(row: Row, event: MouseEvent): void {
  const items = rowMenu(row)
  if (!items.length) return
  activeKey.value = row.key
  menuModel.value = items
  contextMenu.value?.show(event)
}

// cornerEvent builds the MouseEvent a menu opens at from the keyboard or a
// "…" button: never the real click, which openvue/contextmenu's show()
// would stop from propagating.
function cornerEvent(el: Element): MouseEvent {
  const { left, bottom } = el.getBoundingClientRect()
  return new MouseEvent('contextmenu', { clientX: left, clientY: bottom })
}

// openActiveMenu asks for the active row's menu at the row's lower left
// corner, where a platform opens a focused element's menu from the keyboard.
function openActiveMenu(): void {
  const row = activeRow()
  const item = scroller.value?.querySelector('li.active')
  if (!row || !item) return
  openMenu(row, cornerEvent(item))
}

function openMenuFromButton(row: Row, event: MouseEvent): void {
  if (event.currentTarget instanceof Element) openMenu(row, cornerEvent(event.currentTarget))
}

const root = useTemplateRef<HTMLElement>('root')
onMounted(() => root.value?.querySelector('input')?.focus())
</script>

<template>
  <div ref="root" class="picker-list flex min-h-0 flex-1 flex-col gap-1.5">
    <InputText
      v-model="query"
      :placeholder="placeholder"
      :aria-label="placeholder"
      @keydown="onKeydown"
    />
    <div ref="scroller" class="picker-scroll min-h-0 flex-1 overflow-x-hidden overflow-y-auto">
      <ul
        class="picker-rows relative"
        role="listbox"
        :style="{ height: `${virtualizer.getTotalSize()}px`, '--row-height': `${ROW_HEIGHT}px` }"
      >
        <li
          v-for="{ index, start } in virtualizer.getVirtualItems()"
          :key="rows[index]!.key"
          class="absolute inset-x-0 top-0 flex h-(--row-height) min-w-0 items-center gap-1.5 rounded px-1.5"
          :class="[
            rows[index]!.header ? '' : 'cursor-pointer hover:bg-emphasis',
            rows[index]!.key === activeKey && 'active bg-emphasis',
          ]"
          :style="{ transform: `translateY(${start}px)` }"
          :role="rows[index]!.header ? 'presentation' : 'option'"
          :aria-selected="rows[index]!.header ? undefined : rows[index]!.key === activeKey"
          @mousedown.right.stop
          @contextmenu.prevent="openMenu(rows[index]!, $event)"
        >
          <slot name="lead" :row="rows[index]!" />
          <div
            class="flex min-w-0 flex-1 items-center gap-1.5"
            v-bind="rowAttrs?.(rows[index]!)"
            @click="activate(rows[index]!)"
          >
            <slot name="row" :row="rows[index]!" />
          </div>
          <slot name="trail" :row="rows[index]!" />
          <Button
            v-if="menuButton && rowMenu(rows[index]!).length"
            v-bind="ICON_BUTTON"
            :icon="iconClass('rowActions')"
            data-test="row-actions"
            :title="t('picker.rowActions')"
            :aria-label="t('picker.rowActions')"
            aria-haspopup="menu"
            @click="openMenuFromButton(rows[index]!, $event)"
          />
        </li>
      </ul>
    </div>
    <ContextMenu ref="contextMenu" :model="menuModel" />
  </div>
</template>
