<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'

const { t } = useI18n()

type ModelItem = {
  id: string
  object?: string
  created?: number
  owned_by?: string
  display_name?: string
  in_gateway?: boolean
  in_catalog?: boolean
}

type ModelsPayload = {
  items: ModelItem[]
  total: number
  group_id: number
  group_assigned: boolean
  hint?: string
}

const payload = ref<ModelsPayload | null>(null)
const err = ref('')
const q = ref('')

const filtered = computed(() => {
  const items = payload.value?.items ?? []
  const s = q.value.trim().toLowerCase()
  if (!s) return items
  return items.filter(
    (m) =>
      m.id.toLowerCase().includes(s) ||
      (m.owned_by ?? '').toLowerCase().includes(s) ||
      (m.display_name ?? '').toLowerCase().includes(s),
  )
})

async function load() {
  err.value = ''
  try {
    const { data } = await http.get<Envelope<ModelsPayload>>('/api/user/available-models')
    if (data.code !== 0) throw new Error(data.message || t('modelsPage.loadFail'))
    payload.value = data.data
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('modelsPage.loadFail')
  }
}

onMounted(load)
</script>

<template>
  <div class="flex min-h-0 min-w-0 flex-1 flex-col">
    <p class="th-kicker mb-2"><span class="th-kicker-accent">{{ t('modelsPage.kicker') }}</span></p>
    <h1 class="text-3xl font-semibold tracking-tight text-slate-100">{{ t('modelsPage.title') }}</h1>
    <p class="mt-2 max-w-2xl shrink-0 text-sm leading-relaxed text-slate-500">
      {{ t('modelsPage.descBefore') }}<span class="text-slate-400">{{ t('modelsPage.gw') }}</span>{{ t('modelsPage.mid') }}
      <code class="th-dash-code-inline px-1.5 py-0.5 text-xs">GET /v1/models</code>
      {{ t('modelsPage.mid2') }}<span class="text-slate-400">{{ t('modelsPage.cat') }}</span>{{ t('modelsPage.after') }}
    </p>

    <p v-if="err" class="mt-4 shrink-0 rounded-lg border border-red-500/20 bg-red-950/30 px-3 py-2 text-sm text-red-300">{{ err }}</p>

    <div v-if="payload?.hint" class="mt-6 shrink-0 rounded-xl border border-amber-500/20 bg-amber-950/25 px-4 py-3 text-sm text-amber-100/95">
      {{ payload.hint }}
    </div>

    <template v-if="payload">
      <div class="mt-6 flex shrink-0 flex-wrap items-center gap-3">
        <input
          v-model="q"
          type="search"
          :placeholder="t('modelsPage.searchPh')"
          class="th-input max-w-md flex-1 min-w-[200px]"
          :aria-label="t('modelsPage.searchAria')"
        />
        <span class="text-xs text-slate-500">{{ t('modelsPage.count', { total: payload.total, n: filtered.length }) }}</span>
      </div>

      <div class="th-glass mt-6 flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        <div class="th-table-wrap th-table-wrap--scrollable thin-scrollbar">
          <table class="w-full text-left text-sm">
            <thead class="sticky top-0 z-10">
              <tr class="border-b border-white/[0.06] bg-slate-900/95 text-xs uppercase tracking-wider text-slate-500 shadow-[0_1px_0_rgba(0,0,0,0.35)] backdrop-blur-sm">
                <th class="px-4 py-3 font-medium">{{ t('modelsPage.thId') }}</th>
                <th class="px-4 py-3 font-medium">{{ t('modelsPage.thDisplay') }}</th>
                <th class="px-4 py-3 font-medium">{{ t('modelsPage.thSource') }}</th>
                <th class="px-4 py-3 font-medium">{{ t('modelsPage.thOwner') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-white/[0.06]">
              <tr v-for="m in filtered" :key="m.id" class="transition hover:bg-white/[0.03]">
                <td class="px-4 py-3 font-mono text-sm text-cyan-100/95">{{ m.id }}</td>
                <td class="px-4 py-3 text-slate-300">{{ m.display_name || t('modelsPage.dash') }}</td>
                <td class="px-4 py-3">
                  <span v-if="m.in_gateway && m.in_catalog" class="text-xs text-emerald-400/90">{{ t('modelsPage.srcBoth') }}</span>
                  <span v-else-if="m.in_gateway" class="text-xs text-cyan-400/90">{{ t('modelsPage.srcGw') }}</span>
                  <span v-else class="text-xs text-slate-500">{{ t('modelsPage.srcCat') }}</span>
                </td>
                <td class="px-4 py-3 text-slate-400">{{ m.owned_by ?? t('modelsPage.dash') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-if="payload.total === 0" class="px-4 py-12 text-center text-sm text-slate-500">
          {{ t('modelsPage.emptyData') }}
        </p>
        <p v-else-if="filtered.length === 0" class="px-4 py-8 text-center text-sm text-slate-500">{{ t('modelsPage.emptyFilter') }}</p>
      </div>
    </template>
  </div>
</template>
