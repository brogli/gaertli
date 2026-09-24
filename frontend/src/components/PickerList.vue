<script setup lang="ts" generic="Row extends PickerRow">
import { computed, onMounted, ref, useTemplateRef, watch } from 'vue'
import { useVirtualizer } from '@tanstack/vue-virtual'
import InputText from 'openvue/inputtext'
import type { PickerRow } from './pickerRow'

// PickerList is the searchable list of the toolbar pickers: a filter field
// that has focus, the arrow keys move the active row, Enter or a click
// activates a row. A header row only labels a group. The parent filters
// rows by query. Only the rows in view are rendered. The row slot gets
// activate to wire to its own clickable part; a row's action buttons are
// siblings of that part, not nested in it, so a button click never reaches
// it and needs no propagation guard.
const props = defineProps<{ rows: Row[]; placeholder: string }>()
const query = defineModel<string>('query', { default: '' })
const emit = defineEmits<{ activate: [row: Row] }>()
defineSlots<{ row(props: { row: Row; activate: () => void }): unknown }>()

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
  else return
  event.preventDefault()
}

function activateActive(): void {
  const row = props.rows.find((r) => r.key === activeKey.value)
  if (row) emit('activate', row)
}

function activate(row: Row): void {
  if (!row.header) emit('activate', row)
}

const root = useTemplateRef<HTMLElement>('root')
onMounted(() => root.value?.querySelector('input')?.focus())
</script>

<template>
  <div ref="root" class="picker-list">
    <InputText
      v-model="query"
      :placeholder="placeholder"
      :aria-label="placeholder"
      @keydown="onKeydown"
    />
    <div ref="scroller" class="picker-scroll">
      <ul
        class="picker-rows"
        role="listbox"
        :style="{ height: `${virtualizer.getTotalSize()}px`, '--row-height': `${ROW_HEIGHT}px` }"
      >
        <li
          v-for="{ index, start } in virtualizer.getVirtualItems()"
          :key="rows[index]!.key"
          :class="{ header: rows[index]!.header, active: rows[index]!.key === activeKey }"
          :style="{ transform: `translateY(${start}px)` }"
          :role="rows[index]!.header ? 'presentation' : 'option'"
          :aria-selected="rows[index]!.header ? undefined : rows[index]!.key === activeKey"
        >
          <slot name="row" :row="rows[index]!" :activate="() => activate(rows[index]!)" />
        </li>
      </ul>
    </div>
  </div>
</template>

<style scoped>
.picker-list {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 0.4rem;
  min-height: 0;
}
.picker-scroll {
  flex: 1;
  min-height: 0;
  overflow-x: hidden;
  overflow-y: auto;
}
.picker-rows {
  position: relative;
  margin: 0;
  padding: 0;
  list-style: none;
}
.picker-rows li {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  gap: 0.4em;
  height: var(--row-height);
  min-width: 0;
  padding: 0 0.4rem;
  border-radius: 4px;
  cursor: pointer;
}
.picker-rows li:hover,
.picker-rows li.active {
  background: var(--p-content-hover-background);
}
.picker-rows li.header {
  cursor: default;
  background: none;
}
/* The parts of a picker's row, shared by every picker. */
:slotted(.row) {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 0.35em;
  min-width: 0;
}
/* A fixed-width slot so a row without the marker still aligns with one that
   has it. */
:slotted(.current-marker) {
  flex: none;
  width: 1em;
}
:slotted(.muted) {
  font-size: 0.85em;
  color: var(--p-text-muted-color);
}
/* Takes only the space the rest of the row leaves, so it gives way first. */
:slotted(.leftover) {
  flex: 1 1 0;
}
</style>
