<script setup lang="ts">
import axios from 'axios'
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import MobileLabelValueCard from '../components/ui/MobileLabelValueCard.vue'
import MobileLabelValueRow from '../components/ui/MobileLabelValueRow.vue'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import ThTableSkeleton from '../components/ui/ThTableSkeleton.vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'

const { t } = useI18n()

type Log = {
  id: number
  user_id?: number | null
  user_email?: string
  api_key_id?: number | null
  api_key_name?: string
  api_key_prefix?: string
  channel_id?: number | null
  channel_name?: string
  channel_type?: string
  model_upstream?: string | null
  request_id?: string
  model: string
  prompt_tokens: number
  completion_tokens: number
  total_cost: number
  status_code?: number | null
  error_message?: string
  request_time: string
  ip_address?: string
}

const items = ref<Log[]>([])
const total = ref(0)
const { busy: loading, visible: loadingUi, run: runLoad } = useSmoothLoading()
const page = ref(1)
const pageSize = 30
const err = ref('')

const filterUserId = ref('')
const filterEmail = ref('')
const filterModel = ref('')
const filterChannelId = ref('')
const filterIp = ref('')

function httpErr(e: unknown): string {
  if (axios.isAxiosError(e)) {
    if (e.response?.status === 401) return t('login.sessionExpired')
    const d = e.response?.data
    if (d && typeof d === 'object' && 'message' in d) {
      const m = (d as { message?: string }).message
      if (typeof m === 'string' && m.trim()) return m.trim()
    }
  }
  return e instanceof Error ? e.message : t('adminCommon.requestFail')
}

function fmtTime(iso: string) {
  try {
    const d = new Date(iso)
    if (Number.isNaN(d.getTime())) return iso
    return d.toLocaleString()
  } catch {
    return iso
  }
}

function userCell(l: Log): string {
  const id = l.user_id
  const em = (l.user_email ?? '').trim()
  if (id != null && id > 0) {
    return em ? `#${id} · ${em}` : `#${id}`
  }
  return em || '—'
}

function apiKeyCell(l: Log): string {
  const name = (l.api_key_name ?? '').trim()
  const pre = (l.api_key_prefix ?? '').trim()
  const id = l.api_key_id
  if (name) return pre ? `${name} (${pre}…)` : name
  if (pre) return `${pre}…`
  if (id != null && id > 0) return `#${id}`
  return '—'
}

function channelCell(l: Log): string {
  const id = l.channel_id
  const idPart = id != null && id > 0 ? `#${id}` : ''
  const n = (l.channel_name ?? '').trim()
  const ty = (l.channel_type ?? '').trim()
  const meta = [n, ty].filter(Boolean).join(' · ')
  if (idPart && meta) return `${idPart} · ${meta}`
  if (idPart) return idPart
  if (meta) return meta
  return '—'
}

function ridShort(rid: string | undefined): string {
  if (!rid) return '—'
  return rid.length > 18 ? `${rid.slice(0, 10)}…${rid.slice(-6)}` : rid
}

async function load() {
  err.value = ''
  await runLoad(async () => {
    const params: Record<string, string | number> = { page: page.value, page_size: pageSize }
    const uid = filterUserId.value.trim()
    if (uid !== '') params.user_id = uid
    const em = filterEmail.value.trim()
    if (em !== '') params.email = em
    const m = filterModel.value.trim()
    if (m !== '') params.model = m
    const ch = filterChannelId.value.trim()
    if (ch !== '') params.channel_id = ch
    const ip = filterIp.value.trim()
    if (ip !== '') params.ip = ip
    const { data } = await http.get<Envelope<{ items: Log[]; total: number }>>('/api/admin/logs', { params })
    if (data.code !== 0) throw new Error(data.message || t('adminCommon.loadFail'))
    items.value = data.data?.items ?? []
    total.value = data.data?.total ?? 0
  }).catch((e: unknown) => {
    err.value = httpErr(e)
  })
}

function applyFilters() {
  page.value = 1
  void load()
}

function clearFilters() {
  filterUserId.value = ''
  filterEmail.value = ''
  filterModel.value = ''
  filterChannelId.value = ''
  filterIp.value = ''
  page.value = 1
  void load()
}

function prevPage() {
  if (page.value <= 1) return
  page.value -= 1
  void load()
}

function nextPage() {
  if (page.value * pageSize >= total.value) return
  page.value += 1
  void load()
}

const totalPages = () => Math.max(1, Math.ceil(total.value / pageSize))

onMounted(() => void load())
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-bold text-slate-100">{{ t('adminLogs.listHeading') }}</h1>
      <p class="mt-1 max-w-3xl text-sm text-slate-500">{{ t('adminLogs.desc') }}</p>
    </div>

    <div class="flex flex-wrap items-end gap-3">
      <div>
        <label class="mb-1 block text-xs text-slate-500">{{ t('adminLogs.lblFilterUser') }}</label>
        <input v-model="filterUserId" type="text" inputmode="numeric" class="th-input w-28 font-mono text-sm" :placeholder="t('adminLogs.phFilterUser')" />
      </div>
      <div>
        <label class="mb-1 block text-xs text-slate-500">{{ t('adminLogs.lblFilterEmail') }}</label>
        <input v-model="filterEmail" class="th-input w-52 text-sm" :placeholder="t('adminLogs.phFilterEmail')" />
      </div>
      <div>
        <label class="mb-1 block text-xs text-slate-500">{{ t('adminLogs.lblFilterModel') }}</label>
        <input v-model="filterModel" class="th-input w-44 text-sm" :placeholder="t('adminLogs.phFilterModel')" />
      </div>
      <div>
        <label class="mb-1 block text-xs text-slate-500">{{ t('adminLogs.lblFilterChannel') }}</label>
        <input v-model="filterChannelId" type="text" inputmode="numeric" class="th-input w-28 font-mono text-sm" :placeholder="t('adminLogs.phFilterChannel')" />
      </div>
      <div>
        <label class="mb-1 block text-xs text-slate-500">{{ t('adminLogs.lblFilterIp') }}</label>
        <input
          v-model="filterIp"
          class="th-input w-40 font-mono text-sm"
          :placeholder="t('adminLogs.phFilterIp')"
          autocomplete="off"
          spellcheck="false"
        />
      </div>
      <button type="button" class="th-btn-primary-flat px-4 py-2 text-sm" @click="applyFilters">{{ t('adminLogs.apply') }}</button>
      <button type="button" class="rounded-lg border border-white/[0.08] px-4 py-2 text-sm text-slate-300 hover:bg-white/[0.05]" @click="clearFilters">
        {{ t('adminLogs.clear') }}
      </button>
      <button type="button" class="rounded-lg border border-white/[0.08] px-4 py-2 text-sm text-slate-300 hover:bg-white/[0.05]" @click="load">
        {{ t('adminLogs.refresh') }}
      </button>
    </div>

    <p v-if="err" class="text-sm text-rose-400">{{ err }}</p>

    <ThTableSkeleton v-if="loadingUi && items.length === 0" :rows="8" :columns="6" toolbar class="mb-4" />

    <div class="relative hidden overflow-x-auto rounded-lg border border-white/[0.06] lg:block">
      <ThLoadingState v-if="loadingUi && items.length > 0" variant="overlay" />
      <table class="w-full min-w-[88rem] text-left text-xs text-slate-300">
        <thead class="border-b border-white/[0.06] bg-white/[0.02] text-[10px] uppercase tracking-wide text-slate-500">
          <tr>
            <th class="px-3 py-2">{{ t('adminLogs.thTime') }}</th>
            <th class="px-3 py-2">{{ t('adminLogs.thUser') }}</th>
            <th class="px-3 py-2">{{ t('adminLogs.thApiKey') }}</th>
            <th class="px-3 py-2">{{ t('adminLogs.thChannel') }}</th>
            <th class="px-3 py-2">{{ t('adminLogs.thRequestId') }}</th>
            <th class="px-3 py-2">{{ t('adminLogs.thModel') }}</th>
            <th class="px-3 py-2">{{ t('adminLogs.thUpstreamModel') }}</th>
            <th class="px-3 py-2">{{ t('adminLogs.thToken') }}</th>
            <th class="px-3 py-2">{{ t('adminLogs.thCost') }}</th>
            <th class="px-3 py-2">{{ t('adminLogs.thIp') }}</th>
            <th class="px-3 py-2">{{ t('adminLogs.thStatus') }}</th>
            <th class="max-w-[12rem] px-3 py-2">{{ t('adminLogs.thError') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(l, idx) in items" :key="`${page}-${idx}`" class="border-b border-white/[0.04] align-top last:border-0">
            <td class="whitespace-nowrap px-3 py-2 font-mono text-[11px] text-slate-400">{{ fmtTime(l.request_time) }}</td>
            <td class="max-w-[12rem] px-3 py-2 break-all text-[11px] text-slate-400">{{ userCell(l) }}</td>
            <td class="max-w-[10rem] px-3 py-2 break-all text-[11px] text-slate-500">{{ apiKeyCell(l) }}</td>
            <td class="max-w-[10rem] px-3 py-2 break-all text-[11px] text-slate-500">{{ channelCell(l) }}</td>
            <td class="max-w-[8rem] px-3 py-2 font-mono text-[10px] text-slate-500" :title="l.request_id || ''">{{ ridShort(l.request_id) }}</td>
            <td class="max-w-[10rem] truncate px-3 py-2 text-slate-300" :title="l.model">{{ l.model }}</td>
            <td class="max-w-[9rem] truncate px-3 py-2 font-mono text-[10px] text-slate-500" :title="(l.model_upstream ?? '').trim()">
              {{ (l.model_upstream ?? '').trim() || '—' }}
            </td>
            <td class="whitespace-nowrap px-3 py-2 font-mono tabular-nums text-slate-400">{{ l.prompt_tokens }} / {{ l.completion_tokens }}</td>
            <td class="whitespace-nowrap px-3 py-2 font-mono tabular-nums text-slate-300">{{ l.total_cost }}</td>
            <td
              class="max-w-[9rem] truncate px-3 py-2 font-mono text-[10px] text-slate-500"
              :title="(l.ip_address ?? '').trim() || undefined"
            >
              {{ (l.ip_address ?? '').trim() || t('adminCommon.dash') }}
            </td>
            <td class="whitespace-nowrap px-3 py-2 font-mono text-slate-400">{{ l.status_code ?? '—' }}</td>
            <td
              class="max-w-[12rem] truncate px-3 py-2 text-[11px] text-amber-200/80"
              :title="l.error_message || ''"
            >
              {{ l.error_message?.trim() || t('adminCommon.dash') }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="loadingUi && items.length === 0" class="py-8 lg:hidden">
      <ThLoadingState />
    </div>

    <div class="space-y-3 lg:hidden">
      <MobileLabelValueCard v-for="(l, idx) in items" :key="`alog-m-${page}-${idx}`">
        <template #title>
          <p class="font-mono text-[11px] text-slate-400">{{ fmtTime(l.request_time) }}</p>
          <p class="mt-0.5 break-all font-mono text-sm text-slate-200">{{ l.model }}</p>
        </template>
        <MobileLabelValueRow :label="t('adminLogs.thUser')">{{ userCell(l) }}</MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminLogs.thToken')">{{ l.prompt_tokens }} / {{ l.completion_tokens }}</MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminLogs.thCost')">{{ l.total_cost }}</MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminLogs.thStatus')">{{ l.status_code ?? '—' }}</MobileLabelValueRow>
        <MobileLabelValueRow v-if="l.error_message?.trim()" :label="t('adminLogs.thError')">
          <span class="text-amber-200/80">{{ l.error_message }}</span>
        </MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminLogs.thIp')">{{ (l.ip_address ?? '').trim() || t('adminCommon.dash') }}</MobileLabelValueRow>
      </MobileLabelValueCard>
    </div>

    <div class="flex flex-wrap items-center justify-between gap-3 text-xs text-slate-500">
      <span>{{ t('adminLogs.pageInfo', { page, total }) }}</span>
      <div class="flex gap-2">
        <button
          type="button"
          class="rounded-lg border border-white/[0.1] px-3 py-1.5 text-slate-300 transition hover:border-amber-500/30 hover:text-amber-200/90 disabled:opacity-40"
          :disabled="page <= 1"
          @click="prevPage"
        >
          {{ t('adminLogs.prev') }}
        </button>
        <span class="self-center tabular-nums">{{ page }} / {{ totalPages() }}</span>
        <button
          type="button"
          class="rounded-lg border border-white/[0.1] px-3 py-1.5 text-slate-300 transition hover:border-amber-500/30 hover:text-amber-200/90 disabled:opacity-40"
          :disabled="page >= totalPages()"
          @click="nextPage"
        >
          {{ t('adminLogs.next') }}
        </button>
      </div>
    </div>
  </div>
</template>
