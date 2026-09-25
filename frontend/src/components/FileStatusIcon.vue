<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '../i18n'
import { oiClass, statusIcon } from '../lib/icons'

// FileStatusIcon is a changed file's status as an icon in its tone, the
// status named in its tooltip.
const props = defineProps<{ status: string }>()
const { t } = useI18n()
const icon = computed(() => statusIcon(props.status, t))
</script>

<template>
  <span class="status" role="img" :aria-label="icon.label" :title="icon.label">
    <i :class="oiClass(icon.icon)" :style="{ '--nb-tone': `var(--nb-tone-${icon.tone})` }"></i>
  </span>
</template>

<style scoped>
.status {
  flex: none;
  width: 1.2em;
  text-align: center;
}
.status i {
  color: var(--nb-tone);
}
</style>
