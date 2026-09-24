<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from '../i18n'
import type { CoreError } from '../composables/useCoreCall'
import { Code } from '../../bindings/github.com/brogli/nebegit/internal/core'

const props = defineProps<{ error: CoreError }>()
const expanded = ref(false)
const i18n = useI18n()
const { t } = i18n

// headline: the catalogue text for error.key once en.json has an entry for
// it, INTERNAL's fixed text for a bug or other non-git failure, or the
// English message as a fallback (an unrecognised key, or none at all).
const headline = computed(() => {
  if (props.error.code === Code.Internal) return t('errors.INTERNAL')
  const key = props.error.key ? `errors.${props.error.key}` : ''
  return key && i18n.te(key) ? t(key, props.error.args ?? {}) : props.error.message
})

// detail: what "show details" reveals below the headline. An internal
// error has no git command or stderr, so its own message goes there
// instead, untranslated technical detail rather than something to show
// by default.
const detail = computed(() =>
  props.error.code === Code.Internal
    ? props.error.message
    : [props.error.command, props.error.gitStderr].filter(Boolean).join('\n'),
)
</script>

<template>
  <div class="error-banner">
    <p>{{ headline }}</p>
    <!-- an editor start failure's message already names the command -->
    <template v-if="error.code !== Code.EditorNotFound">
      <button type="button" @click="expanded = !expanded">{{ t('notices.showDetails') }}</button>
      <pre v-if="expanded">{{ detail }}</pre>
    </template>
  </div>
</template>

<style scoped>
.error-banner {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  padding: 0.3rem 0.6rem;
  border: 1px solid var(--p-content-border-color);
  border-radius: 4px;
  background: var(--p-content-hover-background);
}
.error-banner p {
  margin: 0;
  white-space: pre-line;
}
.error-banner pre {
  flex-basis: 100%;
  margin: 0;
  white-space: pre-wrap;
}
</style>
