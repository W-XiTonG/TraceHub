<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'

const { t } = useI18n()
const s = ref<Record<string, unknown> | null>(null)

onMounted(async () => {
  const { data } = await http.get<Envelope<Record<string, unknown>>>('/api/admin/dashboard/summary')
  if (data.code === 0) s.value = data.data
})
</script>

<template>
  <div>
    <h1 class="mb-6 text-2xl font-bold">{{ t('adminDash.title') }}</h1>
    <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
      <div class="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <p class="text-xs text-slate-500">{{ t('adminDash.users') }}</p>
        <p class="mt-1 text-2xl font-semibold">{{ s?.total_users ?? t('adminCommon.dash') }}</p>
      </div>
      <div class="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <p class="text-xs text-slate-500">{{ t('adminDash.channels') }}</p>
        <p class="mt-1 text-2xl font-semibold">{{ s?.enabled_channels ?? t('adminCommon.dash') }}</p>
      </div>
      <div class="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <p class="text-xs text-slate-500">{{ t('adminDash.req') }}</p>
        <p class="mt-1 text-2xl font-semibold">{{ s?.today_requests ?? t('adminCommon.dash') }}</p>
      </div>
      <div class="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <p class="text-xs text-slate-500">{{ t('adminDash.tokens') }}</p>
        <p class="mt-1 text-2xl font-semibold">{{ s?.today_tokens ?? t('adminCommon.dash') }}</p>
      </div>
    </div>
  </div>
</template>
