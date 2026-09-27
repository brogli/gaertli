<script setup lang="ts">
// ShortcutHint is a small, muted hint naming a shortcut, pushed to the
// right of whatever it follows in a flex row.
defineProps<{ shortcut: string }>()
</script>

<template>
  <span data-test="shortcut-hint" class="ms-auto text-sm text-muted-color">{{ shortcut }}</span>
</template>
