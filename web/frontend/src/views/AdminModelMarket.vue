<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'

const { t } = useI18n()

type Source = {
  id: number
  name: string
  provider_slug: string
  base_url: string
  models_path: string
  credential_mode: string
  has_static_secret: boolean
  channel_id: number
  enabled: boolean
  last_sync_at?: string
  last_sync_ok: boolean
  last_sync_message?: string
}

type Entry = {
  id: number
  model_id: string
  display_name: string
  provider_slug: string
  owned_by?: string
  description?: string
  source_id?: number
  visible: boolean
  enabled: boolean
  synced_at?: string
}

type Ch = { id: number; channel_name: string; channel_type: string }

const sources = ref<Source[]>([])
const channels = ref<Ch[]>([])
const entries = ref<Entry[]>([])
const entryTotal = ref(0)
const page = ref(1)
const pageSize = 30
const q = ref('')
const msg = ref('')
const err = ref('')

const sourceForm = reactive({
  name: '',
  provider_slug: 'openai',
  base_url: 'https://api.openai.com',
  models_path: '/v1/models',
  credential_mode: 'static',
  api_key: '',
  channel_id: 0,
  enabled: true,
})

const editingEntry = ref<Entry | null>(null)
const editForm = reactive({
  display_name: '',
  description: '',
  visible: true,
  enabled: true,
})

async function loadSources() {
  const { data } = await http.get<Envelope<{ items: Source[] }>>('/api/admin/model-catalog/sources')
  if (data.code === 0) sources.value = data.data.items
}

async function loadChannels() {
  const { data } = await http.get<Envelope<{ items: Ch[] }>>('/api/admin/channels')
  if (data.code === 0) channels.value = data.data.items
}

async function loadEntries() {
  const { data } = await http.get<Envelope<{ items: Entry[]; total: number }>>(
    '/api/admin/model-catalog/entries',
    { params: { page: page.value, page_size: pageSize, q: q.value || undefined } },
  )
  if (data.code === 0) {
    entries.value = data.data.items
    entryTotal.value = data.data.total
  }
}

async function createSource() {
  err.value = ''
  msg.value = ''
  try {
    await http.post('/api/admin/model-catalog/sources', {
      name: sourceForm.name,
      provider_slug: sourceForm.provider_slug,
      base_url: sourceForm.base_url,
      models_path: sourceForm.models_path,
      credential_mode: sourceForm.credential_mode,
      api_key: sourceForm.credential_mode === 'static' ? sourceForm.api_key : undefined,
      channel_id: sourceForm.credential_mode === 'channel' ? sourceForm.channel_id : 0,
      enabled: sourceForm.enabled,
    })
    msg.value = t('adminMarket.sourceSaved')
    sourceForm.api_key = ''
    await loadSources()
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('adminMarket.saveFail')
  }
}

async function syncSource(id: number) {
  err.value = ''
  msg.value = ''
  try {
    const { data } = await http.post<Envelope<{ inserted: number; updated: number }>>(
      `/api/admin/model-catalog/sources/${id}/sync`,
    )
    if (data.code !== 0) throw new Error(data.message)
    msg.value = t('adminMarket.syncDone', { ins: data.data.inserted, upd: data.data.updated })
    await Promise.all([loadSources(), loadEntries()])
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('adminMarket.syncFail')
  }
}

async function removeSource(id: number) {
  if (!confirm(t('adminMarket.confirmDelSource'))) return
  await http.delete(`/api/admin/model-catalog/sources/${id}`)
  await Promise.all([loadSources(), loadEntries()])
}

function openEdit(e: Entry) {
  editingEntry.value = e
  editForm.display_name = e.display_name || e.model_id
  editForm.description = e.description || ''
  editForm.visible = e.visible
  editForm.enabled = e.enabled
}

async function saveEntry() {
  if (!editingEntry.value) return
  await http.put(`/api/admin/model-catalog/entries/${editingEntry.value.id}`, {
    display_name: editForm.display_name,
    description: editForm.description || undefined,
    visible: editForm.visible,
    enabled: editForm.enabled,
  })
  editingEntry.value = null
  await loadEntries()
}

async function removeEntry(id: number) {
  if (!confirm(t('adminMarket.confirmDelEntry'))) return
  await http.delete(`/api/admin/model-catalog/entries/${id}`)
  await loadEntries()
}

onMounted(async () => {
  await Promise.all([loadSources(), loadChannels(), loadEntries()])
})
</script>

<template>
  <div class="space-y-10 text-sm">
    <div>
      <h1 class="mb-1 text-2xl font-bold tracking-tight text-slate-100">{{ t('adminMarket.pageTitle') }}</h1>
      <p class="text-xs text-slate-500">{{ t('adminMarket.internalHint') }}</p>
    </div>

    <p v-if="msg" class="rounded border border-emerald-800 bg-emerald-950/40 px-3 py-2 text-emerald-300">{{ msg }}</p>
    <p v-if="err" class="rounded border border-red-900 bg-red-950/30 px-3 py-2 text-red-300">{{ err }}</p>

    <section class="rounded-xl border border-slate-800 bg-slate-900/60 p-5">
      <h2 class="mb-4 text-lg font-semibold text-slate-200">{{ t('adminMarket.apiSectionTitle') }}</h2>
      <p class="mb-4 text-xs text-slate-500">
        {{ t('adminMarket.apiSectionDesc') }}
      </p>
      <div class="grid max-w-2xl gap-3">
        <label class="flex flex-col gap-1">
          <span class="text-slate-500">{{ t('adminMarket.lblName') }}</span>
          <input v-model="sourceForm.name" class="rounded border border-slate-700 bg-slate-950 px-2 py-1" :placeholder="t('adminMarket.namePh')" />
        </label>
        <label class="flex flex-col gap-1">
          <span class="text-slate-500">{{ t('adminMarket.lblProvider') }}</span>
          <input v-model="sourceForm.provider_slug" class="rounded border border-slate-700 bg-slate-950 px-2 py-1" />
        </label>
        <label class="flex flex-col gap-1">
          <span class="text-slate-500">{{ t('adminMarket.lblBase') }}</span>
          <input v-model="sourceForm.base_url" class="rounded border border-slate-700 bg-slate-950 px-2 py-1" />
        </label>
        <label class="flex flex-col gap-1">
          <span class="text-slate-500">{{ t('adminMarket.lblModelsPath') }}</span>
          <input v-model="sourceForm.models_path" class="rounded border border-slate-700 bg-slate-950 px-2 py-1" />
        </label>
        <label class="flex flex-col gap-1">
          <span class="text-slate-500">{{ t('adminMarket.lblCredMode') }}</span>
          <select v-model="sourceForm.credential_mode" class="rounded border border-slate-700 bg-slate-950 px-2 py-1">
            <option value="none">{{ t('adminMarket.credNone') }}</option>
            <option value="static">{{ t('adminMarket.credStatic') }}</option>
            <option value="channel">{{ t('adminMarket.credChannel') }}</option>
          </select>
        </label>
        <label v-if="sourceForm.credential_mode === 'static'" class="flex flex-col gap-1">
          <span class="text-slate-500">{{ t('adminMarket.lblApiKey') }}</span>
          <input v-model="sourceForm.api_key" type="password" class="rounded border border-slate-700 bg-slate-950 px-2 py-1" />
        </label>
        <label v-if="sourceForm.credential_mode === 'channel'" class="flex flex-col gap-1">
          <span class="text-slate-500">{{ t('adminMarket.lblChannel') }}</span>
          <select v-model.number="sourceForm.channel_id" class="rounded border border-slate-700 bg-slate-950 px-2 py-1">
            <option :value="0">{{ t('adminMarket.pickChannel') }}</option>
            <option v-for="ch in channels" :key="ch.id" :value="ch.id">
              #{{ ch.id }} {{ ch.channel_name || ch.channel_type }}
            </option>
          </select>
        </label>
        <label class="flex items-center gap-2 text-slate-400">
          <input v-model="sourceForm.enabled" type="checkbox" class="rounded border-slate-600" />
          {{ t('adminCommon.enabled') }}
        </label>
        <button
          type="button"
          class="th-btn-primary-flat max-w-xs px-4 py-2 text-sm"
          @click="createSource"
        >
          {{ t('adminMarket.saveSource') }}
        </button>
      </div>

      <h3 class="mb-2 mt-8 font-medium text-slate-300">{{ t('adminMarket.sourcesConfigured') }}</h3>
      <div class="overflow-x-auto">
        <table class="w-full text-left">
          <thead class="text-slate-500">
            <tr>
              <th class="pb-2 pr-3">{{ t('adminUsers.thId') }}</th>
              <th class="pb-2 pr-3">{{ t('adminMarket.lblName') }}</th>
              <th class="pb-2 pr-3">{{ t('adminMarket.thBase') }}</th>
              <th class="pb-2 pr-3">{{ t('adminMarket.thCred') }}</th>
              <th class="pb-2 pr-3">{{ t('adminMarket.thLastSync') }}</th>
              <th class="pb-2"></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-800">
            <tr v-for="s in sources" :key="s.id">
              <td class="py-2 pr-3">{{ s.id }}</td>
              <td class="py-2 pr-3">{{ s.name }}</td>
              <td class="max-w-xs truncate py-2 pr-3 text-xs text-slate-400">{{ s.base_url }}{{ s.models_path }}</td>
              <td class="py-2 pr-3 text-xs">{{ s.credential_mode }}{{ s.channel_id ? ' #' + s.channel_id : '' }}</td>
              <td class="py-2 pr-3 text-xs">
                <span v-if="s.last_sync_at">{{ s.last_sync_ok ? '✓' : '✗' }} {{ s.last_sync_message }}</span>
                <span v-else class="text-slate-600">{{ t('adminMarket.neverSynced') }}</span>
              </td>
              <td class="space-x-2 py-2">
                <button type="button" class="th-link-accent text-sm hover:underline" @click="syncSource(s.id)">{{ t('adminMarket.pullModels') }}</button>
                <button type="button" class="th-link-danger-plain text-sm hover:underline" @click="removeSource(s.id)">{{ t('adminCommon.delete') }}</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="rounded-xl border border-slate-800 bg-slate-900/60 p-5">
      <h2 class="mb-4 text-lg font-semibold text-slate-200">{{ t('adminMarket.catalogTitle') }}</h2>
      <div class="mb-4 flex flex-wrap items-end gap-3">
        <label class="flex flex-col gap-1">
          <span class="text-slate-500">{{ t('adminMarket.search') }}</span>
          <input v-model="q" class="rounded border border-slate-700 bg-slate-950 px-2 py-1" :placeholder="t('adminMarket.searchPh')" />
        </label>
        <button
          type="button"
          class="th-ghost-hover rounded-lg border border-slate-600 px-3 py-1.5 text-sm text-slate-200"
          @click="
            page = 1;
            loadEntries()
          "
        >
          {{ t('adminCommon.query') }}
        </button>
        <span class="text-slate-500">{{ t('adminMarket.totalCount', { n: entryTotal }) }}</span>
      </div>
      <table class="w-full text-left">
        <thead class="text-slate-500">
          <tr>
            <th class="pb-2">{{ t('adminMarket.thModelId') }}</th>
            <th class="pb-2">{{ t('adminMarket.thDisplay') }}</th>
            <th class="pb-2">{{ t('adminMarket.thProvider') }}</th>
            <th class="pb-2">{{ t('adminMarket.thVisible') }}</th>
            <th class="pb-2">{{ t('adminMarket.thEnabled') }}</th>
            <th class="pb-2"></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800">
          <tr v-for="e in entries" :key="e.id">
            <td class="py-2 font-mono text-xs">{{ e.model_id }}</td>
            <td class="py-2">{{ e.display_name }}</td>
            <td class="py-2 text-xs text-slate-400">{{ e.provider_slug }}</td>
            <td class="py-2">{{ e.visible ? t('adminCommon.yes') : t('adminCommon.no') }}</td>
            <td class="py-2">{{ e.enabled ? t('adminCommon.yes') : t('adminCommon.no') }}</td>
            <td class="space-x-2 py-2">
              <button type="button" class="th-link-accent text-sm hover:underline" @click="openEdit(e)">{{ t('adminMarket.editEntry') }}</button>
              <button type="button" class="th-link-danger-plain text-sm hover:underline" @click="removeEntry(e.id)">{{ t('adminCommon.delete') }}</button>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-if="entryTotal > pageSize" class="mt-4 flex gap-2">
        <button
          type="button"
          class="rounded border border-slate-700 px-2 py-1 disabled:opacity-40"
          :disabled="page <= 1"
          @click="
            page--;
            loadEntries()
          "
        >
          {{ t('adminCommon.prevPage') }}
        </button>
        <button
          type="button"
          class="rounded border border-slate-700 px-2 py-1 disabled:opacity-40"
          :disabled="page * pageSize >= entryTotal"
          @click="
            page++;
            loadEntries()
          "
        >
          {{ t('adminCommon.nextPage') }}
        </button>
      </div>
    </section>

    <Transition name="th-modal">
      <div v-if="editingEntry" class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <div class="absolute inset-0 cursor-pointer bg-black/60" aria-hidden="true" @click="editingEntry = null" />
        <div class="th-modal-surface relative z-10 w-full max-w-md rounded-xl border border-slate-700 bg-slate-900 p-5 shadow-xl">
        <h3 class="mb-3 font-semibold">{{ t('adminMarket.editTitle', { id: editingEntry.model_id }) }}</h3>
        <label class="mb-3 flex flex-col gap-1">
          <span class="text-slate-500">{{ t('adminMarket.lblDisplayName') }}</span>
          <input v-model="editForm.display_name" class="rounded border border-slate-700 bg-slate-950 px-2 py-1" />
        </label>
        <label class="mb-3 flex flex-col gap-1">
          <span class="text-slate-500">{{ t('adminMarket.lblDesc') }}</span>
          <textarea v-model="editForm.description" rows="3" class="rounded border border-slate-700 bg-slate-950 px-2 py-1" />
        </label>
        <label class="mb-3 flex items-center gap-2">
          <input v-model="editForm.visible" type="checkbox" /> {{ t('adminMarket.chkVisible') }}
        </label>
        <label class="mb-4 flex items-center gap-2">
          <input v-model="editForm.enabled" type="checkbox" /> {{ t('adminCommon.enabled') }}
        </label>
        <div class="flex justify-end gap-2">
          <button type="button" class="th-ghost-hover rounded-lg border border-slate-600 px-3 py-2 text-sm text-slate-300" @click="editingEntry = null">{{ t('adminCommon.cancel') }}</button>
          <button type="button" class="th-btn-primary-flat px-4 py-2 text-sm" @click="saveEntry">{{ t('adminCommon.save') }}</button>
        </div>
        </div>
      </div>
    </Transition>
  </div>
</template>
