<script setup lang="ts">
import { computed, ref } from 'vue'
import ContextMenu from 'openvue/contextmenu'
import { useI18n } from '@/i18n'

const { t } = useI18n()

const menu = ref<InstanceType<typeof ContextMenu>>()
const target = ref('')

// show opens the menu at event; its one item copies "path:line".
function show(event: MouseEvent, path: string, line: number): void {
  target.value = `${path}:${line}`
  menu.value?.show(event)
}

const items = computed(() => [
  {
    label: t('common.copyPathLine'),
    command: () => void navigator.clipboard.writeText(target.value),
  },
])

defineExpose({ show })
</script>

<template>
  <ContextMenu ref="menu" :model="items" />
</template>
