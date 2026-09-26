<script setup lang="ts" generic="Row extends PickerRow">
import { computed, onMounted, ref, useTemplateRef, watch } from 'vue'
import { useVirtualizer } from '@tanstack/vue-virtual'
import InputText from 'openvue/inputtext'
import type { PickerRow } from './pickerRow'

// PickerList is the searchable list of the toolbar pickers: a filter field
// that has focus, the arrow keys move the active row, Enter or a click
// activates a row. A right-click on a row, or Shift+F10 or the context-menu
// key in the filter for the active row, asks for the row's menu. A header
// row only labels a group. The parent filters rows by query. Only the rows
// in view are rendered. Each row is a clickable part holding the row slot,
// with rowAttrs(row) as its attributes, between the lead and trail slots; a
// button in those slots is a sibling of the clickable part, so its click
// never activates the row. A right-button press stops at its row: OpenVue's
// Popover counts any mousedown in it as a click inside, and a right-click
// sends no click to reset that, so the next click outside would not close
// the list.
const props = defineProps<{
  rows: Row[]
  placeholder: string
  rowAttrs?: (row: Row) => Record<string, string>
}>()
const query = defineModel<string>('query', { default: '' })
const emit = defineEmits<{ activate: [row: Row]; menu: [row: Row, event: MouseEvent] }>()
defineSlots<{
  lead?(props: { row: Row }): unknown
  row(props: { row: Row }): unknown
  trail?(props: { row: Row }): unknown
}>()

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

function openMenu(row: Row, event: MouseEvent): void {
  if (row.header) return
  activeKey.value = row.key
  emit('menu', row, event)
}

// openActiveMenu asks for the active row's menu at the row's lower left
// corner, where a platform opens a focused element's menu from the keyboard.
function openActiveMenu(): void {
  const row = activeRow()
  const item = scroller.value?.querySelector('li.active')
  if (!row || !item) return
  const { left, bottom } = item.getBoundingClientRect()
  emit('menu', row, new MouseEvent('contextmenu', { clientX: left, clientY: bottom }))
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
        </li>
      </ul>
    </div>
  </div>
</template>
