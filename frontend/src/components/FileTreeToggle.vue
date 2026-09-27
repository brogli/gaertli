<script setup lang="ts">
import { computed } from 'vue'
import Button from 'openvue/button'
import { useI18n } from '@/i18n'
import { iconClass } from '@/lib/icons'
import { ICON_BUTTON } from '@/lib/layout'

// FileTreeToggle switches a changed-files list between a flat list and a
// folder tree, asTree its shared, remembered choice (useFileTreeChoice).
const asTree = defineModel<boolean>('asTree', { required: true })
const { t } = useI18n()

const label = computed(() =>
  asTree.value ? t('commitDetailView.showAsList') : t('commitDetailView.showAsTree'),
)
</script>

<template>
  <Button
    v-bind="ICON_BUTTON"
    :text="!asTree"
    :icon="iconClass(asTree ? 'fileTree' : 'fileList')"
    data-test="file-tree-toggle"
    :title="label"
    :aria-label="label"
    :aria-pressed="asTree"
    @click="asTree = !asTree"
  />
</template>
