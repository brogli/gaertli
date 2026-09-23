<script setup lang="ts">
import { computed } from 'vue'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { basename, parentPath } from '../lib/path'

// ChangedFileList renders one row per changed file (status, and oldPath ->
// path for a rename). actionLabel, when set, adds a per-row action button
// (the changes panel's stage/unstage); secondActionLabel adds a second one
// (discard).
const props = defineProps<{
  files: ChangedFile[]
  selected?: ChangedFile | null
  actionLabel?: string
  // | undefined, not just optional: callers (ChangesPanel) bind this from a
  // per-group value that is itself string | undefined, which
  // exactOptionalPropertyTypes treats as distinct from a merely absent prop.
  secondActionLabel?: string | undefined
}>()
const emit = defineEmits<{
  select: [file: ChangedFile]
  open: [file: ChangedFile]
  'open-editor': [file: ChangedFile]
  action: [file: ChangedFile]
  'second-action': [file: ChangedFile]
}>()

// GLYPH is each action's compact button content; title/aria-label carry the
// actual name, since the glyph alone isn't readable as text.
const GLYPH: Record<string, string> = {
  Stage: '+',
  Unstage: '−',
  Discard: '✕',
  'Mark resolved': '✓',
}

// rows splits each path into its directory (rendered muted) and basename
// once per file, rather than in the template on every access.
const rows = computed(() =>
  props.files.map((file) => ({ file, dir: parentPath(file.path), base: basename(file.path) })),
)
</script>

<template>
  <ul class="files">
    <li
      v-for="{ file, dir, base } in rows"
      :key="file.path"
      :class="{ selected: file.path === selected?.path }"
      @click="emit('select', file)"
      @dblclick="emit('open', file)"
    >
      <span class="status">{{ file.status }}</span>
      <span class="path" :title="file.path">
        <template v-if="file.oldPath">{{ file.oldPath }} &rarr; </template>
        <span v-if="dir" class="dir">{{ dir }}/</span>{{ base }}
      </span>
      <span class="actions">
        <button
          type="button"
          class="glyph-btn"
          data-test="open-editor"
          title="Open in editor"
          aria-label="Open in editor"
          @click.stop="emit('open-editor', file)"
        >
          &#9998;
        </button>
        <button
          v-if="actionLabel"
          type="button"
          class="glyph-btn"
          data-test="action"
          :title="actionLabel"
          :aria-label="actionLabel"
          @click.stop="emit('action', file)"
        >
          {{ GLYPH[actionLabel] ?? actionLabel }}
        </button>
        <button
          v-if="secondActionLabel"
          type="button"
          class="glyph-btn"
          data-test="second-action"
          :title="secondActionLabel"
          :aria-label="secondActionLabel"
          @click.stop="emit('second-action', file)"
        >
          {{ GLYPH[secondActionLabel] ?? secondActionLabel }}
        </button>
      </span>
    </li>
  </ul>
</template>

<style scoped>
.files {
  list-style: none;
  margin: 0;
  padding: 0;
}
.files li {
  display: flex;
  align-items: center;
  gap: 0.4em;
  padding: 0.15rem 0.3rem;
  border-radius: 4px;
  cursor: pointer;
}
.files li:hover {
  background: var(--p-content-hover-background);
}
.files li.selected {
  background: var(--p-highlight-background);
  color: var(--p-highlight-color);
}
.status {
  flex: none;
  width: 1.2em;
  font-family: monospace;
  opacity: 0.75;
}
/* The one column that shrinks: actions stay put, ellipsis eats the path. */
.path {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.dir {
  color: var(--p-text-muted-color);
}
.actions {
  flex: none;
  display: flex;
  gap: 0.2em;
  margin-left: auto;
}
</style>
