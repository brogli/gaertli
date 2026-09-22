<script setup lang="ts">
import { ref } from 'vue'
import type { CoreError } from '../composables/useCoreCall'

defineProps<{ error: CoreError }>()
const emit = defineEmits<{ 'open-settings': [] }>()
const expanded = ref(false)
</script>

<template>
  <div class="error-banner">
    <p>{{ error.message }}</p>
    <button type="button" @click="expanded = !expanded">show details</button>
    <button
      v-if="error.code === 'EDITOR_NOT_FOUND'"
      type="button"
      @click="emit('open-settings')"
    >
      Open editor setting
    </button>
    <pre v-if="expanded">{{ error.gitStderr }}</pre>
  </div>
</template>
