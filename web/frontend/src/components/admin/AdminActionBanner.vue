<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  message?: string
  isError?: boolean
  loading?: boolean
  scrollId?: string
}>()

const { t } = useI18n()
</script>

<template>
  <Transition name="th-admin-flash-fade">
    <p
      v-if="loading || message"
      :id="scrollId || 'admin-action-flash'"
      class="rounded-lg border px-4 py-2.5 text-sm"
      :class="loading ? 'border-slate-600/80 bg-slate-800/60 text-slate-300' : isError ? 'th-admin-flash--error' : 'th-admin-flash--success'"
      role="status"
      aria-live="polite"
    >
      <span v-if="loading" class="inline-flex items-center gap-2">
        <span class="inline-block h-3.5 w-3.5 animate-spin rounded-full border-2 border-slate-500 border-t-amber-400" aria-hidden="true" />
        {{ t('adminCommon.saving') }}
      </span>
      <span v-else>{{ message }}</span>
    </p>
  </Transition>
</template>
