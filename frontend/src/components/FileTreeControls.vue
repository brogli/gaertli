<script setup lang="ts">
import { computed } from 'vue'
import Button from 'openvue/button'
import { useI18n } from '@/i18n'
import { iconClass } from '@/lib/icons'
import { ICON_BUTTON } from '@/lib/layout'

// FileTreeControls switches a changed-files list between a flat list and a
// folder tree, asTree its shared, remembered choice (useFileTreeChoice); in
// tree mode a second button folds or unfolds every folder. Each button's
// icon and label show what a click does.
const asTree = defineModel<boolean>('asTree', { required: true })
const folded = defineModel<boolean>('folded', { required: true })
const { t } = useI18n()

const treeLabel = computed(() =>
  asTree.value ? t('commitDetailView.showAsList') : t('commitDetailView.showAsTree'),
)
const foldLabel = computed(() =>
  folded.value ? t('commitDetailView.unfoldAll') : t('commitDetailView.foldAll'),
)
</script>

<template>
  <div class="flex gap-1.5">
    <Button
      v-bind="ICON_BUTTON"
      :text="!asTree"
      :icon="iconClass(asTree ? 'fileList' : 'fileTree')"
      data-test="file-tree-toggle"
      :title="treeLabel"
      :aria-label="treeLabel"
      :aria-pressed="asTree"
      @click="asTree = !asTree"
    />
    <Button
      v-if="asTree"
      v-bind="ICON_BUTTON"
      text
      :icon="iconClass(folded ? 'unfoldAll' : 'foldAll')"
      data-test="fold-toggle"
      :title="foldLabel"
      :aria-label="foldLabel"
      @click="folded = !folded"
    />
  </div>
</template>
