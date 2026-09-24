<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '../i18n'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { basename, parentPath } from '../lib/path'
import { iconClass, oiClass, statusIcon } from '../lib/icons'
import type { ChangedFileAction } from './changedFileAction'

// ChangedFileList renders one row per changed file (status, and oldPath ->
// path for a rename). A click selects a row; a double-click, or Enter on
// the focused row, opens it. action, when set, adds a per-row action button
// (the changes view's stage/unstage); secondAction adds a second one
// (discard). Each names its icon and its own translated label.
const props = defineProps<{
  files: ChangedFile[]
  selected?: ChangedFile | null
  action?: ChangedFileAction
  // | undefined, not just optional: ChangesView binds a per-group value that
  // may be undefined, which exactOptionalPropertyTypes tells from absent.
  secondAction?: ChangedFileAction | undefined
}>()
const { t } = useI18n()
const emit = defineEmits<{
  select: [file: ChangedFile]
  open: [file: ChangedFile]
  'open-editor': [file: ChangedFile]
  action: [file: ChangedFile]
  'second-action': [file: ChangedFile]
}>()

function actionLabel(kind: ChangedFileAction): string {
  return t(`changesView.${kind}`)
}

// rows splits each path into its directory (rendered muted) and basename,
// and resolves its status icon, once per file rather than on every access.
const rows = computed(() =>
  props.files.map((file) => ({
    file,
    dir: parentPath(file.path),
    base: basename(file.path),
    status: statusIcon(file.status, t),
  })),
)
</script>

<template>
  <ul class="files">
    <li
      v-for="{ file, dir, base, status } in rows"
      :key="file.path"
      :class="{ selected: file.path === selected?.path }"
      tabindex="0"
      @click="emit('select', file)"
      @dblclick="emit('open', file)"
      @keydown.enter.self="emit('open', file)"
    >
      <span class="status" role="img" :aria-label="status.label" :title="status.label">
        <i
          :class="oiClass(status.icon)"
          :style="{ '--nb-tone': `var(--nb-tone-${status.tone})` }"
        ></i>
      </span>
      <span class="path" :title="file.path">
        <template v-if="file.oldPath">{{ file.oldPath }} &rarr; </template>
        <span v-if="dir" class="dir">{{ dir }}/</span>{{ base }}
      </span>
      <span class="actions">
        <button
          type="button"
          class="glyph-btn"
          data-test="open-editor"
          :title="t('common.openInEditor')"
          :aria-label="t('common.openInEditor')"
          @click.stop="emit('open-editor', file)"
        >
          <i :class="iconClass('openEditor')" aria-hidden="true"></i>
        </button>
        <button
          v-if="action"
          type="button"
          class="glyph-btn"
          data-test="action"
          :title="actionLabel(action)"
          :aria-label="actionLabel(action)"
          @click.stop="emit('action', file)"
        >
          <i :class="iconClass(action)" aria-hidden="true"></i>
        </button>
        <button
          v-if="secondAction"
          type="button"
          class="glyph-btn"
          data-test="second-action"
          :title="actionLabel(secondAction)"
          :aria-label="actionLabel(secondAction)"
          @click.stop="emit('second-action', file)"
        >
          <i :class="iconClass(secondAction)" aria-hidden="true"></i>
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
  text-align: center;
}
.status i {
  color: var(--nb-tone);
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
