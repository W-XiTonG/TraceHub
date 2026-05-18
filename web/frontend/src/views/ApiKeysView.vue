<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import { copyTextToClipboard } from '../utils/clipboard'
import DateTimeLocalFilterInput from '../components/DateTimeLocalFilterInput.vue'
import MobileLabelValueCard from '../components/ui/MobileLabelValueCard.vue'
import MobileLabelValueRow from '../components/ui/MobileLabelValueRow.vue'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import ThTableSkeleton from '../components/ui/ThTableSkeleton.vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'

const { t } = useI18n()

type KeyRow = {
  id: number
  name?: string
  key_prefix?: string
  /** 服务端是否存有可解密的完整密钥（新建密钥起） */
  can_reveal_full_key?: boolean
  group_name?: string
  status: string
  last_used_at?: string
  usage_24h?: number
  daily_quota?: number
  ip_whitelist?: string[]
  created_at?: string
  expires_at?: string
  log_full_request?: boolean
}

type ExpiryPreset = 'never' | '1h' | '1d' | '30d' | 'custom'

const items = ref<KeyRow[]>([])
const err = ref('')
const createdKey = ref('')
const { busy: listLoading, visible: listLoadingUi, run: runListLoad } = useSmoothLoading()
const loading = ref(false)

const createOpen = ref(false)
const createName = ref('')
const createErr = ref('')
const myGroupName = ref('')
const expiryPreset = ref<ExpiryPreset>('never')
const expiryCustom = ref('')
const dailyUnlimited = ref(true)
const dailyQuotaInput = ref('')
const logFullRequest = ref(false)

const editOpen = ref(false)
const edit = reactive({
  id: 0,
  name: '',
  enabled: true,
  daily_quota: '' as string | number,
  ip_whitelist: '',
})

const filterName = ref('')
const filterPrefix = ref('')
const compact = ref(false)
const selectedIds = ref<number[]>([])
const flash = ref('')

/** 拉取并缓存的完整密钥（仅内存，刷新列表会按 id 清理） */
const fullKeyById = ref<Record<number, string>>({})
/** 是否在表格中明文展示完整密钥 */
const showFullKeyForId = ref<Record<number, boolean>>({})
const revealBusyId = ref<number | null>(null)

const filteredItems = computed(() => {
  const n = filterName.value.trim().toLowerCase()
  const p = filterPrefix.value.trim().toLowerCase()
  return items.value.filter((k) => {
    if (n) {
      const name = (k.name || '').toLowerCase()
      const pref = (k.key_prefix || '').toLowerCase()
      if (!name.includes(n) && !pref.includes(n)) return false
    }
    if (p) {
      const pref = (k.key_prefix || '').toLowerCase()
      if (!pref.includes(p)) return false
    }
    return true
  })
})

const allFilteredSelected = computed(() => {
  const f = filteredItems.value
  if (!f.length) return false
  return f.every((k) => selectedIds.value.includes(k.id))
})

/** datetime-local 最小可选值（本地） */
const minDatetimeLocal = computed(() => {
  const d = new Date()
  d.setMinutes(d.getMinutes() - d.getTimezoneOffset())
  return d.toISOString().slice(0, 16)
})

async function load() {
  await runListLoad(async () => {
    const { data } = await http.get<Envelope<{ items: KeyRow[] }>>('/api/user/apikeys')
    if (data.code === 0) {
      items.value = data.data?.items ?? []
      const idSet = new Set(items.value.map((x) => x.id))
      selectedIds.value = selectedIds.value.filter((id) => idSet.has(id))
      fullKeyById.value = Object.fromEntries(
        Object.entries(fullKeyById.value).filter(([id]) => idSet.has(Number(id))),
      ) as Record<number, string>
      showFullKeyForId.value = Object.fromEntries(
        Object.entries(showFullKeyForId.value).filter(([id]) => idSet.has(Number(id))),
      ) as Record<number, boolean>
    }
  })
}

async function fetchRevealFullKey(id: number): Promise<string> {
  if (fullKeyById.value[id]) return fullKeyById.value[id]
  const { data } = await http.post<Envelope<{ key: string }>>(`/api/user/apikeys/${id}/reveal`, {})
  if (data.code !== 0) throw new Error(data.message || t('apiKeys.revealFail'))
  const k = data.data?.key?.trim()
  if (!k) throw new Error(t('apiKeys.revealFail'))
  fullKeyById.value = { ...fullKeyById.value, [id]: k }
  return k
}

function displayKeyCell(k: KeyRow): string {
  if (showFullKeyForId.value[k.id] && fullKeyById.value[k.id]) return fullKeyById.value[k.id]
  return k.key_prefix || ''
}

async function toggleShowFullKey(k: KeyRow) {
  if (!k.can_reveal_full_key) {
    flash.value = t('apiKeys.revealLegacyHint')
    window.setTimeout(() => {
      if (flash.value === t('apiKeys.revealLegacyHint')) flash.value = ''
    }, 3200)
    return
  }
  err.value = ''
  if (showFullKeyForId.value[k.id]) {
    showFullKeyForId.value = { ...showFullKeyForId.value, [k.id]: false }
    return
  }
  revealBusyId.value = k.id
  try {
    await fetchRevealFullKey(k.id)
    showFullKeyForId.value = { ...showFullKeyForId.value, [k.id]: true }
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('apiKeys.revealFail')
  } finally {
    revealBusyId.value = null
  }
}

async function copyFullSecret(k: KeyRow) {
  err.value = ''
  if (!k.can_reveal_full_key) {
    flash.value = t('apiKeys.revealLegacyHint')
    window.setTimeout(() => {
      if (flash.value === t('apiKeys.revealLegacyHint')) flash.value = ''
    }, 3200)
    return
  }
  try {
    const full = await fetchRevealFullKey(k.id)
    await copyText(full)
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('apiKeys.revealFail')
  }
}

function toggleSelectAll(checked: boolean) {
  if (checked) {
    const set = new Set(selectedIds.value)
    for (const k of filteredItems.value) set.add(k.id)
    selectedIds.value = [...set]
  } else {
    const drop = new Set(filteredItems.value.map((k) => k.id))
    selectedIds.value = selectedIds.value.filter((id) => !drop.has(id))
  }
}

function toggleRow(id: number) {
  if (selectedIds.value.includes(id)) {
    selectedIds.value = selectedIds.value.filter((x) => x !== id)
  } else {
    selectedIds.value = [...selectedIds.value, id]
  }
}

function resetFilters() {
  filterName.value = ''
  filterPrefix.value = ''
  void load()
}

async function runQuery() {
  await load()
}

async function copyText(text: string) {
  if (!text.trim()) return
  try {
    await copyTextToClipboard(text)
    err.value = ''
    flash.value = t('apiKeys.copied')
    window.setTimeout(() => {
      if (flash.value === t('apiKeys.copied')) flash.value = ''
    }, 2200)
  } catch {
    flash.value = ''
    err.value = t('apiKeys.copyFail')
  }
}

function copySelectedPrefixes() {
  const set = new Set(selectedIds.value)
  const lines = items.value.filter((k) => set.has(k.id)).map((k) => k.key_prefix || '').filter(Boolean)
  if (!lines.length) {
    err.value = t('apiKeys.nothingSelected')
    return
  }
  err.value = ''
  void copyText(lines.join('\n'))
}

async function bulkRemove() {
  const ids = [...selectedIds.value]
  if (!ids.length) {
    err.value = t('apiKeys.nothingSelected')
    return
  }
  if (!confirm(t('apiKeys.bulkDeleteConfirm', { n: ids.length }))) return
  err.value = ''
  try {
    for (const id of ids) {
      await http.delete(`/api/user/apikeys/${id}`)
    }
    selectedIds.value = []
    await load()
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('apiKeys.saveFail')
  }
}

async function quickSetEnabled(k: KeyRow, enabled: boolean) {
  err.value = ''
  try {
    const { data } = await http.put<Envelope<null>>(`/api/user/apikeys/${k.id}`, { enabled })
    if (data.code !== 0) throw new Error(data.message || t('apiKeys.saveFail'))
    await load()
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('apiKeys.saveFail')
  }
}

function formatLocal(iso?: string) {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString()
}

function quotaCell(k: KeyRow) {
  const u = k.usage_24h ?? 0
  const cap = k.daily_quota != null && k.daily_quota >= 0 ? String(k.daily_quota) : t('apiKeys.quotaNoCap')
  return `${u} / ${cap}`
}

function ipSummary(k: KeyRow) {
  const list = k.ip_whitelist || []
  if (!list.length) return t('apiKeys.ipUnlimited')
  if (list.length === 1) return list[0]
  return t('apiKeys.ipCount', { n: list.length })
}

function expiresSummary(k: KeyRow) {
  if (!k.expires_at) return t('apiKeys.expiryNever')
  return formatLocal(k.expires_at)
}

function statusBadgeClass(st: string) {
  return st === 'enabled' ? 'th-badge-apikey th-badge-apikey--enabled' : 'th-badge-apikey th-badge-apikey--disabled'
}

async function loadGroup() {
  try {
    const { data } = await http.get<Envelope<{ group: { name?: string } | null }>>('/api/user/group')
    if (data.code === 0 && data.data?.group?.name) myGroupName.value = data.data.group.name.trim()
    else myGroupName.value = ''
  } catch {
    myGroupName.value = ''
  }
}

function chipCls(active: boolean) {
  return active
    ? 'border border-cyan-500/40 bg-cyan-500/15 text-cyan-100'
    : 'th-apikey-chip-idle'
}

function openCreateModal() {
  createErr.value = ''
  createName.value = ''
  expiryPreset.value = 'never'
  expiryCustom.value = ''
  dailyUnlimited.value = true
  dailyQuotaInput.value = ''
  logFullRequest.value = false
  createOpen.value = true
  void loadGroup()
}

function closeCreateModal() {
  createOpen.value = false
  createErr.value = ''
}

function buildExpiresAtISO(): string | undefined {
  const now = Date.now()
  switch (expiryPreset.value) {
    case 'never':
      return undefined
    case '1h':
      return new Date(now + 3600e3).toISOString()
    case '1d':
      return new Date(now + 86400e3).toISOString()
    case '30d':
      return new Date(now + 30 * 86400e3).toISOString()
    case 'custom': {
      if (!expiryCustom.value.trim()) return undefined
      const d = new Date(expiryCustom.value)
      if (Number.isNaN(d.getTime())) return undefined
      return d.toISOString()
    }
    default:
      return undefined
  }
}

async function submitCreateKey() {
  const label = createName.value.trim()
  if (!label) {
    createErr.value = t('apiKeys.nameRequired')
    return
  }
  if (expiryPreset.value === 'custom' && !expiryCustom.value.trim()) {
    createErr.value = t('apiKeys.expiryCustomRequired')
    return
  }
  if (!dailyUnlimited.value) {
    const raw = dailyQuotaInput.value.trim()
    const n = parseInt(raw, 10)
    if (raw === '' || Number.isNaN(n) || n < 0) {
      createErr.value = t('apiKeys.quotaDailyInvalid')
      return
    }
  }

  createErr.value = ''
  createdKey.value = ''
  loading.value = true
  try {
    const body: Record<string, unknown> = { name: label, daily_quota: dailyUnlimited.value ? -1 : parseInt(dailyQuotaInput.value.trim(), 10) }
    const exp = buildExpiresAtISO()
    if (exp) body.expires_at = exp
    if (logFullRequest.value) body.log_full_request = true

    const { data } = await http.post<Envelope<{ key: string; id?: number }>>('/api/user/apikeys', body)
    if (data.code !== 0) throw new Error(data.message || t('apiKeys.createFail'))
    createdKey.value = data.data.key
    closeCreateModal()
    await load()
  } catch (e: unknown) {
    createErr.value = e instanceof Error ? e.message : t('apiKeys.createFail')
    createOpen.value = true
  } finally {
    loading.value = false
  }
}

function openEdit(k: KeyRow) {
  edit.id = k.id
  edit.name = k.name || ''
  edit.enabled = k.status === 'enabled'
  edit.daily_quota = k.daily_quota != null && k.daily_quota >= 0 ? String(k.daily_quota) : k.daily_quota === -1 ? '-1' : ''
  edit.ip_whitelist = (k.ip_whitelist || []).join('\n')
  editOpen.value = true
}

async function saveEdit() {
  err.value = ''
  const lines = edit.ip_whitelist
    .split(/[\n,]+/)
    .map((s) => s.trim())
    .filter(Boolean)
  const body: Record<string, unknown> = {
    name: edit.name.trim() || undefined,
    enabled: edit.enabled,
    ip_whitelist: lines.length ? lines : [],
  }
  const dq = String(edit.daily_quota).trim()
  if (dq !== '') {
    const n = parseInt(dq, 10)
    if (!Number.isNaN(n)) body.daily_quota = n
  }
  try {
    const { data } = await http.put<Envelope<null>>(`/api/user/apikeys/${edit.id}`, body)
    if (data.code !== 0) throw new Error(data.message || t('apiKeys.saveFail'))
    editOpen.value = false
    await load()
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('apiKeys.saveFail')
  }
}

async function remove(id: number) {
  if (!confirm(t('apiKeys.confirmDelete'))) return
  await http.delete(`/api/user/apikeys/${id}`)
  await load()
}

onMounted(() => {
  void load()
  void loadGroup()
})
</script>

<template>
  <div>
    <div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
      <div class="flex min-w-0 items-start gap-3">
        <svg
          class="th-console-section-icon mt-0.5 h-9 w-9 shrink-0"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.75"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <!-- 简洁几何钥匙：避免 Heroicons 单路径弧线在小尺寸下显得抖动 -->
          <circle cx="8.5" cy="12" r="3.25" />
          <path d="M11.75 12H21" />
          <path d="M17 12v3.5" />
          <path d="M17 15.5h3.25" />
        </svg>
        <div class="min-w-0">
          <p class="text-[11px] font-medium uppercase tracking-wider text-slate-500">{{ t('apiKeys.pageKicker') }}</p>
          <h1 class="text-2xl font-bold tracking-tight text-slate-100">{{ t('apiKeys.title') }}</h1>
          <p class="mt-1 max-w-2xl text-sm leading-relaxed text-slate-500">{{ t('apiKeys.desc') }}</p>
        </div>
      </div>
      <button
        type="button"
        class="shrink-0 self-start text-[13px] text-cyan-400/90 underline-offset-4 hover:text-cyan-300 hover:underline"
        @click="compact = !compact"
      >
        {{ compact ? t('apiKeys.standardList') : t('apiKeys.compactList') }}
      </button>
    </div>

    <div
      class="th-console-muted-bar mb-6 flex flex-col gap-3 px-4 py-3 md:flex-row md:flex-wrap md:items-center md:justify-between md:gap-x-4 md:gap-y-2"
    >
      <div class="flex flex-wrap items-center gap-2">
        <button type="button" class="th-btn-primary-flat px-4 py-2 text-sm" @click="openCreateModal">
          {{ t('apiKeys.newKey') }}
        </button>
        <button type="button" class="th-ghost-hover rounded-lg border border-slate-600 px-3 py-2 text-sm text-slate-200" @click="copySelectedPrefixes">
          {{ t('apiKeys.copySelected') }}
        </button>
        <button type="button" class="th-btn-danger-soft px-3 py-2" @click="bulkRemove">
          {{ t('apiKeys.bulkDelete') }}
        </button>
      </div>
      <div
        class="grid w-full min-w-0 grid-cols-1 gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(7rem,11rem)_auto] sm:items-center md:max-w-2xl md:flex-1 lg:max-w-3xl"
      >
        <input
          v-model="filterName"
          type="search"
          :placeholder="t('apiKeys.toolbarSearchPh')"
          class="th-input min-w-0 w-full text-sm"
        />
        <input
          v-model="filterPrefix"
          type="search"
          :placeholder="t('apiKeys.toolbarPrefixPh')"
          class="th-input min-w-0 w-full font-mono text-xs"
        />
        <div class="flex items-stretch gap-2 sm:shrink-0">
          <button
            type="button"
            class="th-ghost-hover min-h-[2.5rem] flex-1 rounded-lg border border-slate-600 px-3 py-2 text-sm text-slate-200 sm:min-h-0 sm:flex-none sm:px-4"
            @click="runQuery"
          >
            {{ t('apiKeys.query') }}
          </button>
          <button
            type="button"
            class="th-ghost-hover min-h-[2.5rem] flex-1 rounded-lg border border-slate-600 px-3 py-2 text-sm text-slate-400 sm:min-h-0 sm:flex-none sm:px-4"
            @click="resetFilters"
          >
            {{ t('apiKeys.reset') }}
          </button>
        </div>
      </div>
    </div>

    <p v-if="flash" class="mb-3 text-sm text-emerald-400">{{ flash }}</p>
    <p v-if="err" class="mb-4 text-sm text-red-400">{{ err }}</p>
    <p class="mb-4 text-[11px] text-slate-600">{{ t('apiKeys.copyPrefixNote') }}</p>

    <ThTableSkeleton v-if="listLoadingUi && items.length === 0" :rows="8" :columns="6" toolbar class="mb-4" />
    <div v-if="listLoadingUi && items.length === 0" class="py-8 lg:hidden">
      <ThLoadingState />
    </div>

    <div v-if="createdKey" class="mb-6 rounded-xl border border-amber-600/50 bg-amber-950/40 p-4 text-sm">
      <p class="font-medium text-amber-400">{{ t('apiKeys.saveOnce') }}</p>
      <code class="th-dash-code-inline mt-2 block break-all px-2 py-1.5 text-xs">{{ createdKey }}</code>
    </div>

    <div class="th-user-keys-table-wrap relative hidden overflow-x-auto lg:block">
      <ThLoadingState v-if="listLoadingUi && items.length > 0" variant="overlay" />
      <table class="w-full min-w-[56rem] text-left" :class="compact ? 'text-xs' : 'text-sm'">
        <thead class="text-slate-500">
          <tr>
            <th class="w-10 px-3 py-2.5" :class="compact ? 'py-1.5' : ''">
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-slate-600 bg-slate-950 text-cyan-500"
                :checked="allFilteredSelected"
                @change="toggleSelectAll(($event.target as HTMLInputElement).checked)"
              />
            </th>
            <th class="px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('apiKeys.thName') }}</th>
            <th class="px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('apiKeys.thStatus') }}</th>
            <th class="px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('apiKeys.thQuota') }}</th>
            <th v-if="!compact" class="hidden px-3 py-2.5 font-medium md:table-cell">{{ t('apiKeys.thGroup') }}</th>
            <th class="min-w-[9rem] px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('apiKeys.thPrefix') }}</th>
            <th v-if="!compact" class="hidden px-3 py-2.5 font-medium lg:table-cell">{{ t('apiKeys.thModels') }}</th>
            <th v-if="!compact" class="hidden px-3 py-2.5 font-medium xl:table-cell">{{ t('apiKeys.thIp') }}</th>
            <th v-if="!compact" class="hidden px-3 py-2.5 font-medium xl:table-cell">{{ t('apiKeys.thCreated') }}</th>
            <th v-if="!compact" class="hidden px-3 py-2.5 font-medium xl:table-cell">{{ t('apiKeys.thExpires') }}</th>
            <th class="px-3 py-2.5 text-right font-medium" :class="compact ? 'py-1.5' : ''">{{ t('apiKeys.thActions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="k in filteredItems" :key="k.id">
            <td class="px-3" :class="compact ? 'py-1.5' : 'py-2.5'">
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-slate-600 bg-slate-950 text-cyan-500"
                :checked="selectedIds.includes(k.id)"
                @change="toggleRow(k.id)"
              />
            </td>
            <td class="px-3 font-medium text-slate-200" :class="compact ? 'py-1.5' : 'py-2.5'">{{ k.name || t('apiKeys.dash') }}</td>
            <td class="px-3" :class="compact ? 'py-1.5' : 'py-2.5'">
              <span :class="statusBadgeClass(k.status)">
                {{ k.status === 'enabled' ? t('apiKeys.statusEnabled') : t('apiKeys.statusDisabled') }}
              </span>
            </td>
            <td class="px-3 font-mono text-slate-400" :class="compact ? 'py-1.5 text-[11px]' : 'py-2.5 text-xs'">{{ quotaCell(k) }}</td>
            <td v-if="!compact" class="hidden px-3 text-xs text-slate-500 md:table-cell" :class="compact ? '' : 'py-2.5'">
              {{ k.group_name || t('apiKeys.dash') }}
            </td>
            <td class="px-3" :class="compact ? 'py-1.5' : 'py-2.5'">
              <div class="flex max-w-[14rem] flex-col gap-1">
                <div class="flex items-start gap-1">
                  <code
                    class="min-w-0 flex-1 font-mono text-[11px] leading-snug text-cyan-200/90"
                    :class="showFullKeyForId[k.id] ? 'break-all' : 'truncate'"
                  >{{ displayKeyCell(k) }}</code>
                  <button
                    type="button"
                    class="th-ghost-hover shrink-0 rounded p-1 text-slate-500 hover:text-cyan-400 disabled:opacity-40"
                    :title="t('apiKeys.copyFullKey')"
                    :disabled="revealBusyId === k.id"
                    @click="copyFullSecret(k)"
                  >
                    <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M15.666 3.888A2.25 2.25 0 0013.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 01-.75.75H9a.75.75 0 01-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 01-2.25 2.25H6.75A2.25 2.25 0 014.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 011.927-.184" />
                    </svg>
                  </button>
                </div>
                <button
                  type="button"
                  class="self-start text-[10px] font-medium text-slate-500 underline-offset-2 hover:text-cyan-400 hover:underline disabled:opacity-40"
                  :disabled="revealBusyId === k.id"
                  @click="toggleShowFullKey(k)"
                >
                  {{
                    revealBusyId === k.id
                      ? t('apiKeys.revealLoading')
                      : showFullKeyForId[k.id]
                        ? t('apiKeys.revealHide')
                        : t('apiKeys.revealShow')
                  }}
                </button>
              </div>
            </td>
            <td v-if="!compact" class="hidden px-3 text-xs text-slate-500 lg:table-cell" :class="compact ? '' : 'py-2.5'">
              {{ t('apiKeys.modelsNA') }}
            </td>
            <td v-if="!compact" class="hidden max-w-[8rem] truncate px-3 text-xs text-slate-500 xl:table-cell" :class="compact ? '' : 'py-2.5'" :title="ipSummary(k)">
              {{ ipSummary(k) }}
            </td>
            <td v-if="!compact" class="hidden whitespace-nowrap px-3 text-xs text-slate-500 xl:table-cell" :class="compact ? '' : 'py-2.5'">
              {{ k.created_at ? formatLocal(k.created_at) : t('apiKeys.dash') }}
            </td>
            <td v-if="!compact" class="hidden whitespace-nowrap px-3 text-xs text-slate-500 xl:table-cell" :class="compact ? '' : 'py-2.5'">
              {{ expiresSummary(k) }}
            </td>
            <td class="px-3 text-right" :class="compact ? 'py-1.5' : 'py-2.5'">
              <div class="flex flex-wrap justify-end gap-x-2 gap-y-1">
                <button
                  v-if="k.status === 'enabled'"
                  type="button"
                  class="th-btn-danger-soft !text-[11px] !px-2 !py-0.5 !leading-snug"
                  @click="quickSetEnabled(k, false)"
                >
                  {{ t('apiKeys.disableBtn') }}
                </button>
                <button
                  v-else
                  type="button"
                  class="rounded border border-emerald-500/35 px-2 py-0.5 text-[11px] font-medium text-emerald-300 hover:bg-emerald-950/30"
                  @click="quickSetEnabled(k, true)"
                >
                  {{ t('apiKeys.enableBtn') }}
                </button>
                <button type="button" class="text-[11px] text-cyan-400 hover:underline" @click="openEdit(k)">{{ t('apiKeys.editBtn') }}</button>
                <button type="button" class="th-link-danger-plain text-[11px]" @click="remove(k.id)">{{ t('apiKeys.deleteBtn') }}</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="!filteredItems.length" class="px-4 py-8 text-center text-sm text-slate-500">
        {{ items.length ? t('apiKeys.noMatch') : t('apiKeys.emptyList') }}
      </p>
    </div>

    <div class="space-y-3 lg:hidden">
      <MobileLabelValueCard v-for="k in filteredItems" :key="`key-m-${k.id}`">
        <template #title>
          <div class="flex items-start gap-2">
            <input
              type="checkbox"
              class="mt-0.5 h-4 w-4 shrink-0 rounded border-slate-600 bg-slate-950 text-cyan-500"
              :checked="selectedIds.includes(k.id)"
              @change="toggleRow(k.id)"
            />
            <span class="min-w-0 font-medium text-slate-200">{{ k.name || t('apiKeys.dash') }}</span>
          </div>
        </template>
        <MobileLabelValueRow :label="t('apiKeys.thStatus')">
          <span :class="statusBadgeClass(k.status)">{{ k.status === 'enabled' ? t('apiKeys.statusEnabled') : t('apiKeys.statusDisabled') }}</span>
        </MobileLabelValueRow>
        <MobileLabelValueRow :label="t('apiKeys.thQuota')">
          <span class="font-mono text-xs">{{ quotaCell(k) }}</span>
        </MobileLabelValueRow>
        <MobileLabelValueRow v-if="!compact" :label="t('apiKeys.thGroup')">{{ k.group_name || t('apiKeys.dash') }}</MobileLabelValueRow>
        <MobileLabelValueRow :label="t('apiKeys.thPrefix')">
          <code class="break-all font-mono text-[11px] text-cyan-200/90">{{ displayKeyCell(k) }}</code>
        </MobileLabelValueRow>
        <div class="flex flex-wrap gap-2 pt-1">
          <button type="button" class="text-[10px] text-slate-500 underline-offset-2 hover:text-cyan-400 hover:underline" :disabled="revealBusyId === k.id" @click="toggleShowFullKey(k)">
            {{ revealBusyId === k.id ? t('apiKeys.revealLoading') : showFullKeyForId[k.id] ? t('apiKeys.revealHide') : t('apiKeys.revealShow') }}
          </button>
          <button type="button" class="text-[10px] text-cyan-400 hover:underline" :disabled="revealBusyId === k.id" @click="copyFullSecret(k)">{{ t('apiKeys.copyFullKey') }}</button>
        </div>
        <div class="flex flex-wrap gap-2 border-t border-white/[0.06] pt-2">
          <button v-if="k.status === 'enabled'" type="button" class="th-btn-danger-soft !text-[11px] !px-2 !py-0.5" @click="quickSetEnabled(k, false)">{{ t('apiKeys.disableBtn') }}</button>
          <button v-else type="button" class="rounded border border-emerald-500/35 px-2 py-0.5 text-[11px] font-medium text-emerald-300" @click="quickSetEnabled(k, true)">{{ t('apiKeys.enableBtn') }}</button>
          <button type="button" class="text-[11px] text-cyan-400 hover:underline" @click="openEdit(k)">{{ t('apiKeys.editBtn') }}</button>
          <button type="button" class="th-link-danger-plain text-[11px]" @click="remove(k.id)">{{ t('apiKeys.deleteBtn') }}</button>
        </div>
      </MobileLabelValueCard>
      <p v-if="!filteredItems.length" class="py-8 text-center text-sm text-slate-500">{{ items.length ? t('apiKeys.noMatch') : t('apiKeys.emptyList') }}</p>
    </div>

    <Teleport to="body">
      <Transition name="th-modal">
        <div
          v-if="createOpen"
          class="fixed inset-0 z-[130] flex items-center justify-center p-4 pb-[max(1rem,calc(env(safe-area-inset-bottom,0px)+1rem))] sm:p-6"
          role="dialog"
          aria-modal="true"
          aria-labelledby="create-key-modal-title"
        >
          <div
            class="absolute inset-0 cursor-pointer bg-black/65 backdrop-blur-[2px]"
            aria-hidden="true"
            @click="closeCreateModal"
          />
          <div
            class="th-modal-surface th-modal-card-flat relative z-10 flex max-h-[min(92dvh,52rem)] w-full max-w-lg flex-col overflow-hidden rounded-2xl border border-slate-700/90 bg-slate-900"
          >
            <header class="th-modal-divider-b flex shrink-0 items-start justify-between gap-4 px-5 py-4">
              <div class="min-w-0">
                <h2 id="create-key-modal-title" class="text-lg font-semibold tracking-tight text-slate-100">
                  {{ t('apiKeys.createTitle') }}
                </h2>
                <p class="mt-1 text-[13px] leading-relaxed text-slate-500">{{ t('apiKeys.createDrawerSubtitle') }}</p>
              </div>
              <button
                type="button"
                class="th-ghost-hover shrink-0 rounded-lg p-2 text-slate-400 transition hover:text-slate-200"
                :aria-label="t('apiKeys.closeDrawer')"
                @click="closeCreateModal"
              >
                <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </header>

            <div class="thin-scrollbar min-h-0 flex-1 overflow-y-auto overscroll-y-contain px-5 py-5">
              <div class="space-y-5">
              <section class="th-console-panel p-4">
                <div class="mb-4 flex gap-3">
                  <svg
                    class="th-console-section-icon mt-0.5 h-8 w-8 shrink-0"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.75"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    aria-hidden="true"
                  >
                    <circle cx="8.5" cy="12" r="3.25" />
                    <path d="M11.75 12H21" />
                    <path d="M17 12v3.5" />
                    <path d="M17 15.5h3.25" />
                  </svg>
                  <div class="min-w-0">
                    <h3 class="text-sm font-semibold text-slate-100">{{ t('apiKeys.sectionBasic') }}</h3>
                    <p class="mt-0.5 text-xs leading-relaxed text-slate-500">{{ t('apiKeys.sectionBasicDesc') }}</p>
                  </div>
                </div>

                <label class="block">
                  <span class="text-[13px] text-slate-400">
                    {{ t('apiKeys.nameLabel') }} <span class="text-red-400">*</span>
                  </span>
                  <input
                    v-model="createName"
                    type="text"
                    maxlength="128"
                    autocomplete="off"
                    :placeholder="t('apiKeys.namePh')"
                    class="th-input mt-1.5 w-full"
                    @keydown.enter.prevent="submitCreateKey"
                  />
                </label>

                <div class="th-surface-faint mt-4 px-3 py-2.5">
                  <p class="text-[11px] font-medium uppercase tracking-wide text-slate-500">{{ t('apiKeys.routingGroup') }}</p>
                  <p v-if="myGroupName" class="mt-1 text-[13px] leading-relaxed text-slate-400">
                    {{ t('apiKeys.routingGroupHint', { name: myGroupName }) }}
                  </p>
                  <p v-else class="mt-1 text-[13px] leading-relaxed text-amber-200/85">{{ t('apiKeys.routingGroupUnknown') }}</p>
                </div>

                <div class="mt-4">
                  <p class="mb-2 text-[13px] text-slate-400">{{ t('apiKeys.expiryLabel') }}</p>
                  <div class="flex flex-wrap gap-2">
                    <button type="button" :class="['rounded-lg px-3 py-1.5 text-xs font-medium transition', chipCls(expiryPreset === 'never')]" @click="expiryPreset = 'never'">
                      {{ t('apiKeys.expiryNever') }}
                    </button>
                    <button type="button" :class="['rounded-lg px-3 py-1.5 text-xs font-medium transition', chipCls(expiryPreset === '1h')]" @click="expiryPreset = '1h'">
                      {{ t('apiKeys.expiry1h') }}
                    </button>
                    <button type="button" :class="['rounded-lg px-3 py-1.5 text-xs font-medium transition', chipCls(expiryPreset === '1d')]" @click="expiryPreset = '1d'">
                      {{ t('apiKeys.expiry1d') }}
                    </button>
                    <button type="button" :class="['rounded-lg px-3 py-1.5 text-xs font-medium transition', chipCls(expiryPreset === '30d')]" @click="expiryPreset = '30d'">
                      {{ t('apiKeys.expiry30d') }}
                    </button>
                    <button type="button" :class="['rounded-lg px-3 py-1.5 text-xs font-medium transition', chipCls(expiryPreset === 'custom')]" @click="expiryPreset = 'custom'">
                      {{ t('apiKeys.expiryCustom') }}
                    </button>
                  </div>
                  <div v-if="expiryPreset === 'custom'" class="mt-3">
                    <DateTimeLocalFilterInput
                      v-model="expiryCustom"
                      input-id="api-keys-expiry-custom"
                      :aria-label="t('apiKeys.expiryCustom')"
                      :min="minDatetimeLocal"
                      variant="field"
                      wrapper-class="max-w-none"
                    />
                    <p class="mt-1.5 text-[11px] text-slate-600">{{ t('apiKeys.expiryCustomHint') }}</p>
                  </div>
                </div>
              </section>

              <section class="th-console-panel p-4">
                <div class="mb-4 flex gap-3">
                  <svg
                    class="th-console-section-icon mt-0.5 h-8 w-8"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.5"
                    aria-hidden="true"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M21 12a2.25 2.25 0 00-2.25-2.25H15a3 3 0 11-6 0H5.25A2.25 2.25 0 003 12m18 0v6a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 18v-6m18 0V9M3 12V9m18 3a2.25 2.25 0 01-2.25-2.25H15a3 3 0 10-6 0H5.25A2.25 2.25 0 003 12"
                    />
                  </svg>
                  <div class="min-w-0">
                    <h3 class="text-sm font-semibold text-slate-100">{{ t('apiKeys.sectionQuota') }}</h3>
                    <p class="mt-0.5 text-xs leading-relaxed text-slate-500">{{ t('apiKeys.sectionQuotaDesc') }}</p>
                  </div>
                </div>

                <label class="flex cursor-pointer items-start gap-3 rounded-lg border border-transparent py-1 transition hover:border-slate-600/35">
                  <input v-model="dailyUnlimited" type="checkbox" class="mt-1 h-4 w-4 shrink-0 rounded border-slate-600 bg-slate-950 text-cyan-500 focus:ring-cyan-500/30" />
                  <span class="min-w-0">
                    <span class="block text-[13px] font-medium text-slate-200">{{ t('apiKeys.quotaUnlimited') }}</span>
                    <span class="mt-0.5 block text-[11px] leading-relaxed text-slate-500">{{ t('apiKeys.quotaUnlimitedHint') }}</span>
                  </span>
                </label>

                <div v-if="!dailyUnlimited" class="mt-4">
                  <label class="block">
                    <span class="text-[13px] text-slate-400">{{ t('apiKeys.quotaDailyTokens') }}</span>
                    <input
                      v-model="dailyQuotaInput"
                      type="number"
                      min="0"
                      step="1"
                      inputmode="numeric"
                      class="th-input mt-1.5 w-full font-mono text-sm"
                    />
                  </label>
                </div>
              </section>

              <section class="th-console-panel p-4">
                <div class="mb-3 flex gap-3">
                  <svg
                    class="th-console-section-icon mt-0.5 h-8 w-8"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.5"
                    aria-hidden="true"
                  >
                    <path stroke-linecap="round" stroke-linejoin="round" d="M2.036 12.322a1.012 1.012 0 010-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178z" />
                    <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                  </svg>
                  <div class="min-w-0">
                    <h3 class="text-sm font-semibold text-slate-100">{{ t('apiKeys.sectionObserve') }}</h3>
                  </div>
                </div>
                <label class="flex cursor-pointer items-start gap-3">
                  <input v-model="logFullRequest" type="checkbox" class="mt-1 h-4 w-4 shrink-0 rounded border-slate-600 bg-slate-950 text-cyan-500 focus:ring-cyan-500/30" />
                  <span class="min-w-0">
                    <span class="block text-[13px] font-medium text-slate-200">{{ t('apiKeys.logFullReq') }}</span>
                    <span class="mt-0.5 block text-[11px] leading-relaxed text-slate-500">{{ t('apiKeys.logFullReqHint') }}</span>
                  </span>
                </label>
              </section>
              </div>

              <p v-if="createErr" class="mt-5 rounded-lg border border-red-500/25 bg-red-950/35 px-3 py-2 text-sm text-red-300">{{ createErr }}</p>
            </div>

            <footer class="th-modal-divider-t th-modal-footer-bar flex shrink-0 justify-end gap-2 px-5 py-4 backdrop-blur-sm">
              <button type="button" class="th-ghost-hover rounded-lg border border-slate-600 px-4 py-2 text-sm text-slate-300" @click="closeCreateModal">
                {{ t('apiKeys.cancel') }}
              </button>
              <button
                type="button"
                :disabled="loading"
                class="th-btn-primary-flat inline-flex items-center gap-2 px-4 py-2 text-sm disabled:opacity-50"
                @click="submitCreateKey"
              >
                <svg class="h-4 w-4 opacity-90" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z" />
                </svg>
                {{ loading ? t('apiKeys.creating') : t('apiKeys.submitCreate') }}
              </button>
            </footer>
          </div>
        </div>
      </Transition>
    </Teleport>

    <Teleport to="body">
      <Transition name="th-modal">
        <div v-if="editOpen" class="fixed inset-0 z-[130] flex items-center justify-center p-4 sm:p-6">
          <div class="absolute inset-0 cursor-pointer bg-black/65 backdrop-blur-[2px]" aria-hidden="true" @click="editOpen = false" />
          <div class="th-modal-surface th-modal-card-flat relative z-10 w-full max-w-md rounded-xl border border-slate-700 bg-slate-900 p-6">
          <h3 class="mb-4 font-semibold">{{ t('apiKeys.editTitle') }}</h3>
          <label class="mb-3 block text-sm">
            <span class="text-slate-500">{{ t('apiKeys.labelName') }}</span>
            <input v-model="edit.name" class="th-input mt-1 w-full" />
          </label>
          <label class="mb-3 flex items-center gap-2 text-sm">
            <input v-model="edit.enabled" type="checkbox" /> {{ t('apiKeys.enabled') }}
          </label>
          <label class="mb-3 block text-sm">
            <span class="text-slate-500">{{ t('apiKeys.dailyQuota') }}</span>
            <input v-model="edit.daily_quota" :placeholder="t('apiKeys.dailyQuotaPh')" class="th-input mt-1 w-full font-mono text-xs" />
          </label>
          <label class="mb-4 block text-sm">
            <span class="text-slate-500">{{ t('apiKeys.ipWhitelist') }}</span>
            <textarea v-model="edit.ip_whitelist" rows="3" class="th-input mt-1 w-full font-mono text-xs" />
          </label>
          <div class="flex justify-end gap-2">
            <button type="button" class="th-ghost-hover rounded border border-slate-600 px-3 py-1.5 text-sm" @click="editOpen = false">{{ t('apiKeys.cancel') }}</button>
            <button type="button" class="th-btn-primary-flat px-3 py-1.5 text-sm" @click="saveEdit">{{ t('apiKeys.save') }}</button>
          </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>
