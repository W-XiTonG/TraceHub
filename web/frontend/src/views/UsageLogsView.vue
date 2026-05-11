<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'

const { t } = useI18n()

type LogRow = {
  id: number
  request_id?: string
  model: string
  channel_id?: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  total_cost: number
  request_time: string
  response_time_ms?: number
  status_code?: number
  error_message?: string
}

const items = ref<LogRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const err = ref('')
const filters = reactive({
  model: '',
  status_code: '',
  start_time: '',
  end_time: '',
})

async function load() {
  err.value = ''
  try {
    const { data } = await http.get<Envelope<{ items: LogRow[]; total: number }>>('/api/user/usage-logs', {
      params: {
        page: page.value,
        page_size: pageSize,
        model: filters.model || undefined,
        status_code: filters.status_code || undefined,
        start_time: filters.start_time || undefined,
        end_time: filters.end_time || undefined,
      },
    })
    if (data.code !== 0) throw new Error(data.message || t('modelsPage.loadFail'))
    items.value = data.data.items
    total.value = data.data.total
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('modelsPage.loadFail')
  }
}

function applyFilters() {
  page.value = 1
  load()
}

function resetFilters() {
  filters.model = ''
  filters.status_code = ''
  filters.start_time = ''
  filters.end_time = ''
  applyFilters()
}

onMounted(load)
</script>

<template>
  <div>
    <h1 class="mb-2 text-2xl font-bold">{{ t('logsPage.title') }}</h1>
    <p class="mb-6 text-sm text-slate-500">{{ t('logsPage.desc') }}</p>

    <div class="mb-6 flex flex-wrap items-end gap-3 rounded-xl border border-slate-800 bg-slate-900/60 p-4">
      <label class="flex flex-col gap-1 text-xs">
        <span class="text-slate-500">{{ t('logsPage.model') }}</span>
        <input v-model="filters.model" :placeholder="t('logsPage.modelPh')" class="rounded border border-slate-700 bg-slate-950 px-2 py-1.5 text-sm" />
      </label>
      <label class="flex flex-col gap-1 text-xs">
        <span class="text-slate-500">{{ t('logsPage.status') }}</span>
        <input v-model="filters.status_code" :placeholder="t('logsPage.statusPh')" class="w-20 rounded border border-slate-700 bg-slate-950 px-2 py-1.5 text-sm" />
      </label>
      <label class="flex flex-col gap-1 text-xs">
        <span class="text-slate-500">{{ t('logsPage.start') }}</span>
        <input v-model="filters.start_time" class="w-44 rounded border border-slate-700 bg-slate-950 px-2 py-1.5 text-sm" />
      </label>
      <label class="flex flex-col gap-1 text-xs">
        <span class="text-slate-500">{{ t('logsPage.end') }}</span>
        <input v-model="filters.end_time" class="w-44 rounded border border-slate-700 bg-slate-950 px-2 py-1.5 text-sm" />
      </label>
      <button type="button" class="rounded-lg bg-cyan-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-cyan-500" @click="applyFilters">{{ t('logsPage.filterBtn') }}</button>
      <button type="button" class="rounded-lg border border-slate-600 px-3 py-1.5 text-sm hover:bg-slate-800" @click="resetFilters">{{ t('logsPage.resetBtn') }}</button>
    </div>

    <p v-if="err" class="mb-4 text-sm text-red-400">{{ err }}</p>

    <div class="overflow-x-auto rounded-xl border border-slate-800">
      <table class="w-full min-w-[800px] text-left text-sm">
        <thead class="bg-slate-900 text-slate-500">
          <tr>
            <th class="px-3 py-2">{{ t('logsPage.thTime') }}</th>
            <th class="px-3 py-2">{{ t('logsPage.model') }}</th>
            <th class="px-3 py-2">{{ t('logsPage.thTokCol') }}</th>
            <th class="px-3 py-2">{{ t('logsPage.thCost') }}</th>
            <th class="px-3 py-2">{{ t('logsPage.thStatus') }}</th>
            <th class="px-3 py-2">{{ t('logsPage.thLatency') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800 bg-slate-950/50">
          <tr v-for="row in items" :key="row.id">
            <td class="whitespace-nowrap px-3 py-2 text-xs text-slate-400">{{ row.request_time }}</td>
            <td class="px-3 py-2 font-mono text-xs">{{ row.model }}</td>
            <td class="px-3 py-2 text-xs">{{ row.total_tokens }}</td>
            <td class="px-3 py-2 text-xs">{{ row.total_cost }}</td>
            <td class="px-3 py-2 text-xs">{{ row.status_code ?? t('modelsPage.dash') }}</td>
            <td class="px-3 py-2 text-xs">{{ row.response_time_ms ?? t('modelsPage.dash') }}</td>
          </tr>
          <tr v-if="!items.length">
            <td colspan="6" class="px-3 py-8 text-center text-slate-500">{{ t('logsPage.empty') }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="total > pageSize" class="mt-4 flex items-center gap-3 text-sm text-slate-400">
      <button
        type="button"
        class="rounded border border-slate-700 px-2 py-1 disabled:opacity-40"
        :disabled="page <= 1"
        @click="
          page--;
          load()
        "
      >
        {{ t('logsPage.prev') }}
      </button>
      <span>{{ t('logsPage.pageInfo', { page, total }) }}</span>
      <button
        type="button"
        class="rounded border border-slate-700 px-2 py-1 disabled:opacity-40"
        :disabled="page * pageSize >= total"
        @click="
          page++;
          load()
        "
      >
        {{ t('logsPage.next') }}
      </button>
    </div>
  </div>
</template>
