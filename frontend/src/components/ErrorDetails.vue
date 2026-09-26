<script setup lang="ts">
import { ref } from 'vue'
import Button from 'openvue/button'
import { useI18n } from '@/i18n'
import type { CoreError } from '@/composables/useCoreCall'
import { errorDetail, hasDetail } from '@/lib/errorText'

// ErrorDetails is an error's "Show details" toggle and what it reveals,
// for a flex-wrap row parent: the details take a row of their own.
defineProps<{ error: CoreError }>()
const expanded = ref(false)
const { t } = useI18n()
</script>

<template>
  <template v-if="hasDetail(error)">
    <Button
      :label="t('notices.showDetails')"
      link
      size="small"
      :aria-expanded="expanded"
      @click="expanded = !expanded"
    />
    <pre v-if="expanded" class="basis-full whitespace-pre-wrap">{{ errorDetail(error) }}</pre>
  </template>
</template>
