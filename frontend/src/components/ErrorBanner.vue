<script setup lang="ts">
import { ref } from 'vue'
import type { CoreError } from '../composables/useCoreCall'

defineProps<{ error: CoreError }>()
const expanded = ref(false)
</script>

<template>
  <div class="error-banner">
    <p>{{ error.message }}</p>
    <button type="button" @click="expanded = !expanded">show details</button>
    <pre v-if="expanded">{{ error.gitStderr }}</pre>
  </div>
</template>
