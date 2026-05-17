<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import ThLoadingSpinner from '../ui/ThLoadingSpinner.vue'

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
        <ThLoadingSpinner size="sm" tone="muted" />
        {{ t('adminCommon.saving') }}
      </span>
      <span v-else>{{ message }}</span>
    </p>
  </Transition>
</template>
