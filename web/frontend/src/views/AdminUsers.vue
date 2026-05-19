<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { storeToRefs } from 'pinia'
import { http, type Envelope } from '../api/http'
import { copyTextToClipboard } from '../utils/clipboard'
import { useAuthStore } from '../stores/auth'
import MobileLabelValueCard from '../components/ui/MobileLabelValueCard.vue'
import MobileLabelValueRow from '../components/ui/MobileLabelValueRow.vue'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import ThTableSkeleton from '../components/ui/ThTableSkeleton.vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'

const { t } = useI18n()
const auth = useAuthStore()
const { role: authRole } = storeToRefs(auth)

type Row = {
  id: number
  email: string
  balance: number
  status: number
  group_name?: string
  role?: string
}

type GroupOpt = { id: number; name: string }

type UserDetail = {
  id: number
  email: string
  username?: string
  group_id: number
  group_name?: string
  balance: number
  status: number
  role: string
  concurrency: number
  rate_multiplier: number
  balance_alert_threshold: number
  webhook_url?: string
  ip_whitelist?: string[]
  ip_blacklist?: string[]
  totp_enabled: boolean
}

type LedgerAdminRow = {
  id: number
  user_id: number
  user_email: string
  redeem_code_id: number
  amount: number
  balance_after: number
  code_hint?: string | null
  created_at?: string
}

const items = ref<Row[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(30)
const { busy: loading, visible: loadingUi, run: runLoad } = useSmoothLoading()
const deltaByUser = ref<Record<number, string>>({})
const filterText = ref('')
const filterApplied = ref('')
const compact = ref(false)
const selectedIds = ref<number[]>([])
const flash = ref('')
const err = ref('')

const groups = ref<GroupOpt[]>([])
const editOpen = ref(false)
const { busy: editLoading, visible: editLoadingUi, run: runEditLoad } = useSmoothLoading({
  showDelay: 200,
  minVisible: 280,
})
const editSaving = ref(false)
const editForm = reactive({
  id: 0,
  email: '',
  username: '',
  groupIdStr: '' as string,
  balance: 0,
  status: 1,
  role: 'user',
  concurrency: 0,
  rateMultiplier: 1,
  balanceAlert: 0,
  webhookUrl: '',
  ipWhitelistText: '',
  ipBlacklistText: '',
  newPassword: '',
  clearTotp: false,
  totpEnabled: false,
})

const canManageAdmins = computed(() => authRole.value === 'superadmin')

const userLedgerRows = ref<LedgerAdminRow[]>([])
const userLedgerTotal = ref(0)
const userLedgerErr = ref('')

function rowIsElevated(u: Row) {
  return u.role === 'admin' || u.role === 'superadmin'
}

const totalPages = computed(() => {
  if (total.value <= 0) return 1
  return Math.max(1, Math.ceil(total.value / pageSize.value))
})

const allFilteredSelected = computed(() => {
  const f = items.value
  if (!f.length) return false
  return f.every((u) => selectedIds.value.includes(u.id))
})

function toggleSelectAll(checked: boolean) {
  if (checked) {
    const set = new Set(selectedIds.value)
    for (const u of items.value) set.add(u.id)
    selectedIds.value = [...set]
  } else {
    const drop = new Set(items.value.map((u) => u.id))
    selectedIds.value = selectedIds.value.filter((id) => !drop.has(id))
  }
}

function listQueryParams(): Record<string, string | number> {
  const params: Record<string, string | number> = {
    page: page.value,
    page_size: pageSize.value,
  }
  const q = filterApplied.value.trim()
  if (q) {
    if (/^\d+$/.test(q)) {
      params.user_id = q
    } else {
      params.email = q
    }
  }
  return params
}

function toggleRow(id: number) {
  if (selectedIds.value.includes(id)) {
    selectedIds.value = selectedIds.value.filter((x) => x !== id)
  } else {
    selectedIds.value = [...selectedIds.value, id]
  }
}

async function load() {
  err.value = ''
  await runLoad(async () => {
    const { data } = await http.get<
      Envelope<{ items: Row[]; total: number; page: number; page_size: number }>
    >('/api/admin/users', { params: listQueryParams() })
    if (data.code !== 0) throw new Error(data.message || t('adminCommon.loadFail'))
    items.value = data.data?.items ?? []
    total.value = typeof data.data?.total === 'number' ? data.data.total : 0
    if (typeof data.data?.page === 'number' && data.data.page >= 1) {
      page.value = data.data.page
    }
    const idSet = new Set(items.value.map((x) => x.id))
    selectedIds.value = selectedIds.value.filter((id) => idSet.has(id))
  }).catch((e: unknown) => {
    err.value = e instanceof Error ? e.message : t('adminCommon.loadFail')
  })
}

function applyFilters() {
  filterApplied.value = filterText.value.trim()
  page.value = 1
  void load()
}

function resetFilter() {
  filterText.value = ''
  filterApplied.value = ''
  page.value = 1
  void load()
}

function onPageSizeChange() {
  page.value = 1
  void load()
}

function prevPage() {
  if (page.value <= 1) return
  page.value -= 1
  void load()
}

function nextPage() {
  if (page.value >= totalPages.value) return
  page.value += 1
  void load()
}

async function adjust(id: number) {
  const raw = deltaByUser.value[id]
  const delta = parseInt(raw || '0', 10)
  if (!delta) return
  err.value = ''
  try {
    await http.post(`/api/admin/users/${id}/balance`, { delta })
    deltaByUser.value[id] = ''
    await load()
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('adminCommon.requestFail')
  }
}

async function removeUser(u: Row) {
  if (!confirm(t('adminUsers.confirmDelete', { email: u.email }))) return
  err.value = ''
  try {
    const { data } = await http.delete<Envelope<unknown>>(`/api/admin/users/${u.id}`)
    if (data.code !== 0) throw new Error(data.message || t('adminCommon.requestFail'))
    await load()
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('adminCommon.requestFail')
  }
}

function joinLines(arr: string[] | undefined | null) {
  return (arr || []).join('\n')
}

function parseIpLines(s: string): string[] {
  return s
    .split(/[\n,]+/)
    .map((x) => x.trim())
    .filter(Boolean)
}

async function loadUserLedger(uid: number) {
  userLedgerErr.value = ''
  userLedgerRows.value = []
  userLedgerTotal.value = 0
  try {
    const { data: ld } = await http.get<Envelope<{ items: LedgerAdminRow[]; total: number }>>('/api/admin/redeem-ledger', {
      params: { user_id: uid, page: 1, page_size: 12 },
    })
    if (ld.code !== 0) throw new Error(ld.message || t('adminCommon.requestFail'))
    userLedgerRows.value = ld.data?.items ?? []
    userLedgerTotal.value = ld.data?.total ?? 0
  } catch (e: unknown) {
    userLedgerErr.value = e instanceof Error ? e.message : t('adminCommon.requestFail')
  }
}

async function openEdit(u: Row) {
  err.value = ''
  editOpen.value = true
  editForm.newPassword = ''
  editForm.clearTotp = false
  await runEditLoad(async () => {
    if (!groups.value.length) {
      const { data: gd } = await http.get<Envelope<{ items: GroupOpt[] }>>('/api/admin/groups')
      if (gd.code !== 0) throw new Error(gd.message || t('adminCommon.loadFail'))
      groups.value = gd.data?.items || []
    }
    const { data } = await http.get<Envelope<UserDetail>>(`/api/admin/users/${u.id}`)
    if (data.code !== 0) throw new Error(data.message || t('adminCommon.loadFail'))
    const d = data.data
    editForm.id = d.id
    editForm.email = d.email || ''
    editForm.username = d.username || ''
    editForm.groupIdStr = d.group_id ? String(d.group_id) : ''
    editForm.balance = Number(d.balance)
    editForm.status = d.status
    editForm.role = d.role || 'user'
    editForm.concurrency = d.concurrency ?? 0
    editForm.rateMultiplier = d.rate_multiplier ?? 1
    editForm.balanceAlert = Number(d.balance_alert_threshold ?? 0)
    editForm.webhookUrl = d.webhook_url || ''
    editForm.ipWhitelistText = joinLines(d.ip_whitelist)
    editForm.ipBlacklistText = joinLines(d.ip_blacklist)
    editForm.totpEnabled = !!d.totp_enabled
    void loadUserLedger(d.id)
  }).catch((e: unknown) => {
    err.value = e instanceof Error ? e.message : t('adminCommon.loadFail')
    editOpen.value = false
  })
}

function closeEdit() {
  editOpen.value = false
  userLedgerRows.value = []
  userLedgerTotal.value = 0
  userLedgerErr.value = ''
}

async function saveEdit() {
  err.value = ''
  editSaving.value = true
  try {
    const body: Record<string, unknown> = {
      email: editForm.email.trim(),
      username: editForm.username.trim(),
      balance: Math.trunc(editForm.balance),
      status: editForm.status,
      role: editForm.role,
      concurrency: Math.trunc(editForm.concurrency),
      rate_multiplier: editForm.rateMultiplier,
      balance_alert_threshold: Math.trunc(editForm.balanceAlert),
      webhook_url: editForm.webhookUrl.trim(),
      ip_whitelist: parseIpLines(editForm.ipWhitelistText),
      ip_blacklist: parseIpLines(editForm.ipBlacklistText),
    }
    if (editForm.groupIdStr === '') {
      body.no_group = true
    } else {
      const gid = parseInt(editForm.groupIdStr, 10)
      if (Number.isNaN(gid) || gid <= 0) {
        err.value = t('adminUsers.invalidGroup')
        return
      }
      body.group_id = gid
    }
    if (editForm.newPassword.trim()) {
      body.new_password = editForm.newPassword.trim()
    }
    if (editForm.clearTotp) {
      body.clear_totp = true
    }
    const { data } = await http.put<Envelope<unknown>>(`/api/admin/users/${editForm.id}`, body)
    if (data.code !== 0) throw new Error(data.message || t('adminCommon.requestFail'))
    flash.value = t('adminUsers.editSaved')
    window.setTimeout(() => {
      if (flash.value === t('adminUsers.editSaved')) flash.value = ''
    }, 2200)
    closeEdit()
    await load()
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('adminCommon.requestFail')
  } finally {
    editSaving.value = false
  }
}

function statusBadge(u: Row) {
  return u.status === 1 ? 'th-badge-apikey th-badge-apikey--enabled' : 'th-badge-apikey th-badge-apikey--disabled'
}

function statusLabel(u: Row) {
  return u.status === 1 ? t('adminUsers.statusActive') : t('adminUsers.statusInactive')
}

async function copyEmails() {
  const set = new Set(selectedIds.value)
  const emails = items.value.filter((u) => set.has(u.id)).map((u) => u.email).filter(Boolean)
  if (!emails.length) {
    err.value = t('adminUsers.nothingSelected')
    return
  }
  err.value = ''
  try {
    await copyTextToClipboard(emails.join('\n'))
    flash.value = t('adminUsers.copied')
    window.setTimeout(() => {
      if (flash.value === t('adminUsers.copied')) flash.value = ''
    }, 2200)
  } catch {
    flash.value = t('adminUsers.copyFail')
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <div>
    <div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
      <div class="flex min-w-0 items-start gap-3">
        <span class="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-sky-500/12 text-sky-300 ring-1 ring-sky-500/25">
          <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M15 19.128a9.38 9.38 0 002.625.372 9.337 9.337 0 004.121-.952 4.125 4.125 0 00-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 018.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0111.964-3.07M12 6.375a3.375 3.375 0 11-6.75 0 3.375 3.375 0 016.75 0zm8.25 2.25a2.625 2.625 0 11-5.25 0 2.625 2.625 0 015.25 0z"
            />
          </svg>
        </span>
        <div class="min-w-0">
          <p class="text-[11px] font-medium uppercase tracking-wider text-slate-500">{{ t('adminUsers.pageKicker') }}</p>
          <h1 class="text-2xl font-bold tracking-tight text-slate-100">{{ t('adminUsers.title') }}</h1>
          <p class="mt-1 max-w-2xl text-sm leading-relaxed text-slate-500">{{ t('adminUsers.desc') }}</p>
        </div>
      </div>
      <button
        type="button"
        class="th-link-accent shrink-0 self-start text-[13px] underline-offset-4 hover:underline"
        @click="compact = !compact"
      >
        {{ compact ? t('adminUsers.standardList') : t('adminUsers.compactList') }}
      </button>
    </div>

    <div class="th-console-muted-bar th-console-muted-bar--sm mb-4 flex flex-col gap-3 px-4 py-3 sm:flex-row sm:flex-wrap sm:items-center">
      <input
        v-model="filterText"
        type="search"
        :placeholder="t('adminUsers.searchPh')"
        class="th-input min-w-[12rem] flex-1 text-sm sm:max-w-xs"
        @keydown.enter.prevent="applyFilters"
      />
      <button type="button" class="th-btn-primary-flat shrink-0 px-3 py-2 text-sm" :disabled="loading" @click="applyFilters">
        {{ t('adminCommon.query') }}
      </button>
      <button type="button" class="th-ghost-hover shrink-0 rounded-lg border border-slate-600 px-3 py-2 text-sm text-slate-200" :disabled="loading" @click="resetFilter">
        {{ t('adminUsers.resetFilter') }}
      </button>
      <button type="button" class="th-ghost-hover shrink-0 rounded-lg border border-slate-600 px-3 py-2 text-sm text-slate-200" @click="copyEmails">
        {{ t('adminUsers.copyEmails') }}
      </button>
    </div>

    <div
      class="mb-6 flex flex-col gap-3 overflow-visible rounded-lg border border-slate-800/80 bg-slate-900/40 px-3 py-2.5 sm:flex-row sm:flex-wrap sm:items-center sm:gap-x-1 sm:gap-y-2"
    >
      <div class="flex shrink-0 items-center gap-2">
        <span class="text-xs text-slate-500">{{ t('adminGlobalPricing.pageSize') }}</span>
        <select
          v-model.number="pageSize"
          class="th-select th-select--inline w-[4.25rem] !max-w-none py-0 pr-6 text-xs"
          :disabled="loading"
          @change="onPageSizeChange"
        >
          <option :value="20">20</option>
          <option :value="30">30</option>
          <option :value="50">50</option>
          <option :value="100">100</option>
        </select>
      </div>

      <div class="hidden h-6 w-px shrink-0 bg-slate-700/90 sm:block" aria-hidden="true" />

      <div class="flex min-w-0 flex-1 flex-wrap items-center justify-end gap-2 sm:flex-none sm:basis-auto">
        <span
          class="min-w-0 truncate text-right text-xs tabular-nums text-slate-400 sm:max-w-[20rem]"
          :title="`${t('adminGlobalPricing.pageTotal', { total })} · ${t('adminGlobalPricing.pageStatus', { page, pages: totalPages })}`"
        >
          <span class="text-slate-300">{{ t('adminGlobalPricing.pageTotal', { total }) }}</span>
          <span class="mx-1.5 text-slate-600" aria-hidden="true">·</span>
          <span class="font-mono text-slate-300">{{
            t('adminGlobalPricing.pageStatus', { page, pages: totalPages })
          }}</span>
        </span>
        <div class="flex shrink-0 items-center gap-1">
          <button
            type="button"
            class="h-9 rounded-lg border border-slate-600 px-2.5 text-xs text-slate-200 hover:bg-slate-800 disabled:opacity-40"
            :disabled="page <= 1 || loading"
            @click="prevPage"
          >
            {{ t('adminCommon.prevPage') }}
          </button>
          <button
            type="button"
            class="h-9 rounded-lg border border-slate-600 px-2.5 text-xs text-slate-200 hover:bg-slate-800 disabled:opacity-40"
            :disabled="page >= totalPages || loading"
            @click="nextPage"
          >
            {{ t('adminCommon.nextPage') }}
          </button>
        </div>
      </div>
    </div>

    <p v-if="flash" class="mb-3 text-sm text-emerald-400">{{ flash }}</p>
    <p v-if="err" class="mb-4 text-sm text-red-400">{{ err }}</p>

    <ThTableSkeleton v-if="loadingUi && items.length === 0" :rows="8" :columns="6" toolbar class="mb-4" />
    <div v-if="loadingUi && items.length === 0" class="py-8 lg:hidden">
      <ThLoadingState />
    </div>

    <div class="th-user-keys-table-wrap relative hidden overflow-x-auto lg:block">
      <ThLoadingState v-if="loadingUi && items.length > 0" variant="overlay" />
      <table class="w-full min-w-[640px] text-left" :class="compact ? 'text-xs' : 'text-sm'">
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
            <th class="px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('adminUsers.thId') }}</th>
            <th class="px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('adminUsers.thEmail') }}</th>
            <th class="px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('adminUsers.thStatus') }}</th>
            <th class="px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('adminUsers.thGroup') }}</th>
            <th class="px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('adminUsers.thBalance') }}</th>
            <th class="px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('adminUsers.thDelta') }}</th>
            <th class="px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('adminUsers.thActions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in items" :key="u.id">
            <td class="px-3" :class="compact ? 'py-1.5' : 'py-2.5'">
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-slate-600 bg-slate-950 text-cyan-500"
                :checked="selectedIds.includes(u.id)"
                @change="toggleRow(u.id)"
              />
            </td>
            <td class="px-3 font-mono text-slate-400" :class="compact ? 'py-1.5' : 'py-2.5'">{{ u.id }}</td>
            <td class="px-3 text-slate-200" :class="compact ? 'py-1.5' : 'py-2.5'">{{ u.email }}</td>
            <td class="px-3" :class="compact ? 'py-1.5' : 'py-2.5'">
              <span :class="statusBadge(u)">
                {{ statusLabel(u) }}
              </span>
            </td>
            <td class="px-3 text-slate-400" :class="compact ? 'py-1.5' : 'py-2.5'">{{ u.group_name || t('adminCommon.dash') }}</td>
            <td class="px-3 font-mono text-slate-300" :class="compact ? 'py-1.5' : 'py-2.5'">{{ u.balance }}</td>
            <td class="px-3" :class="compact ? 'py-1.5' : 'py-2.5'">
              <div class="flex flex-wrap items-center gap-2">
                <input
                  v-model="deltaByUser[u.id]"
                  type="number"
                  :placeholder="t('adminUsers.deltaPh')"
                  class="th-input !w-24 shrink-0 py-1.5 text-xs"
                />
                <button type="button" class="text-[11px] th-link-accent hover:underline" @click="adjust(u.id)">{{ t('adminUsers.apply') }}</button>
              </div>
            </td>
            <td class="px-3" :class="compact ? 'py-1.5' : 'py-2.5'">
              <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
                <button
                  type="button"
                  class="text-[11px] th-link-accent hover:underline disabled:cursor-not-allowed disabled:opacity-40"
                  :disabled="rowIsElevated(u) && !canManageAdmins"
                  @click="openEdit(u)"
                >
                  {{ t('adminUsers.edit') }}
                </button>
                <button
                  type="button"
                  class="th-link-danger-plain text-[11px] font-medium disabled:cursor-not-allowed"
                  :disabled="auth.user?.id === u.id"
                  @click="removeUser(u)"
                >
                  {{ t('adminCommon.delete') }}
                </button>
              </div>
            </td>
          </tr>
          <tr v-if="!items.length && !loadingUi && filterApplied">
            <td colspan="8" class="px-3 py-8 text-center text-slate-500">{{ t('adminUsers.noMatch') }}</td>
          </tr>
          <tr v-if="!items.length && !loadingUi && !filterApplied">
            <td colspan="8" class="px-3 py-8 text-center text-slate-500">{{ t('adminCommon.dash') }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="space-y-3 lg:hidden">
      <MobileLabelValueCard v-for="u in items" :key="`au-m-${u.id}`" :title="u.email">
        <template #title>
          <div class="flex items-start gap-2">
            <input
              type="checkbox"
              class="mt-0.5 h-4 w-4 shrink-0 rounded border-slate-600 bg-slate-950 text-cyan-500"
              :checked="selectedIds.includes(u.id)"
              @change="toggleRow(u.id)"
            />
            <span class="min-w-0 break-all font-medium text-slate-200">{{ u.email }}</span>
          </div>
        </template>
        <MobileLabelValueRow :label="t('adminUsers.thId')">{{ u.id }}</MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminUsers.thStatus')">
          <span :class="statusBadge(u)">{{ statusLabel(u) }}</span>
        </MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminUsers.thGroup')">{{ u.group_name || t('adminCommon.dash') }}</MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminUsers.thBalance')">
          <span class="font-mono">{{ u.balance }}</span>
        </MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminUsers.thDelta')">
          <div class="flex flex-wrap items-center gap-2">
            <input v-model="deltaByUser[u.id]" type="number" :placeholder="t('adminUsers.deltaPh')" class="th-input !w-24 shrink-0 py-1.5 text-xs" />
            <button type="button" class="text-[11px] th-link-accent hover:underline" @click="adjust(u.id)">{{ t('adminUsers.apply') }}</button>
          </div>
        </MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminUsers.thActions')">
          <div class="flex flex-wrap gap-2">
            <button
              type="button"
              class="text-[11px] th-link-accent hover:underline disabled:cursor-not-allowed disabled:opacity-40"
              :disabled="rowIsElevated(u) && !canManageAdmins"
              @click="openEdit(u)"
            >
              {{ t('adminUsers.edit') }}
            </button>
            <button
              type="button"
              class="th-link-danger-plain text-[11px] font-medium disabled:cursor-not-allowed"
              :disabled="auth.user?.id === u.id"
              @click="removeUser(u)"
            >
              {{ t('adminCommon.delete') }}
            </button>
          </div>
        </MobileLabelValueRow>
      </MobileLabelValueCard>
      <p v-if="!items.length && !loadingUi && filterApplied" class="py-8 text-center text-sm text-slate-500">{{ t('adminUsers.noMatch') }}</p>
      <p v-if="!items.length && !loadingUi && !filterApplied" class="py-8 text-center text-sm text-slate-500">{{ t('adminCommon.dash') }}</p>
    </div>

    <Teleport to="body">
      <div
        v-if="editOpen"
        class="fixed inset-0 z-50 flex cursor-pointer items-center justify-center bg-black/60 p-4 backdrop-blur-sm"
        @click.self="closeEdit"
      >
        <div
          class="max-h-[90vh] w-full max-w-2xl cursor-default overflow-y-auto rounded-2xl border border-slate-700 bg-slate-900 p-6 shadow-2xl"
          role="dialog"
          aria-modal="true"
        >
          <h3 class="mb-1 text-lg font-semibold text-white">{{ t('adminUsers.editModalTitle') }}</h3>
          <p class="mb-4 text-xs text-slate-500">{{ t('adminUsers.editModalHint') }}</p>
          <ThLoadingState v-if="editLoadingUi" />
          <template v-else>
            <div class="grid gap-4 text-sm sm:grid-cols-2">
              <label class="flex flex-col gap-1 sm:col-span-2">
                <span class="text-xs text-slate-400">{{ t('adminUsers.lblEmail') }}</span>
                <input v-model="editForm.email" type="email" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2" />
              </label>
              <label class="flex flex-col gap-1 sm:col-span-2">
                <span class="text-xs text-slate-400">{{ t('adminUsers.lblUsername') }}</span>
                <input v-model="editForm.username" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2" />
              </label>
              <label class="flex flex-col gap-1">
                <span class="text-xs text-slate-400">{{ t('adminUsers.lblGroup') }}</span>
                <select v-model="editForm.groupIdStr" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2">
                  <option value="">{{ t('adminUsers.noGroup') }}</option>
                  <option v-for="g in groups" :key="g.id" :value="String(g.id)">{{ g.name }}</option>
                </select>
              </label>
              <label class="flex flex-col gap-1">
                <span class="text-xs text-slate-400">{{ t('adminUsers.lblStatus') }}</span>
                <select v-model.number="editForm.status" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2">
                  <option :value="1">{{ t('adminUsers.statusActive') }}</option>
                  <option :value="2">{{ t('adminUsers.statusInactive') }}</option>
                </select>
              </label>
              <label class="flex flex-col gap-1">
                <span class="text-xs text-slate-400">{{ t('adminUsers.lblRole') }}</span>
                <select v-model="editForm.role" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2">
                  <option value="user">{{ t('adminUsers.roleUser') }}</option>
                  <option value="admin">{{ t('adminUsers.roleAdmin') }}</option>
                  <option v-if="canManageAdmins" value="superadmin">{{ t('adminUsers.roleSuper') }}</option>
                </select>
              </label>
              <label class="flex flex-col gap-1">
                <span class="text-xs text-slate-400">{{ t('adminUsers.lblBalance') }}</span>
                <input v-model.number="editForm.balance" type="number" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono" />
              </label>
              <label class="flex flex-col gap-1">
                <span class="text-xs text-slate-400">{{ t('adminUsers.lblConcurrency') }}</span>
                <input v-model.number="editForm.concurrency" type="number" min="0" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono" />
              </label>
              <label class="flex flex-col gap-1">
                <span class="text-xs text-slate-400">{{ t('adminUsers.lblRateMul') }}</span>
                <input v-model.number="editForm.rateMultiplier" type="number" step="0.01" min="0.01" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono" />
              </label>
              <label class="flex flex-col gap-1">
                <span class="text-xs text-slate-400">{{ t('adminUsers.lblBalAlert') }}</span>
                <input v-model.number="editForm.balanceAlert" type="number" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono" />
              </label>
              <label class="flex flex-col gap-1 sm:col-span-2">
                <span class="text-xs text-slate-400">{{ t('adminUsers.lblWebhook') }}</span>
                <input v-model="editForm.webhookUrl" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs" />
              </label>
              <label class="flex flex-col gap-1 sm:col-span-2">
                <span class="text-xs text-slate-400">{{ t('adminUsers.lblIpWhite') }}</span>
                <textarea
                  v-model="editForm.ipWhitelistText"
                  rows="3"
                  class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs"
                  :placeholder="t('adminUsers.ipListPh')"
                />
              </label>
              <label class="flex flex-col gap-1 sm:col-span-2">
                <span class="text-xs text-slate-400">{{ t('adminUsers.lblIpBlack') }}</span>
                <textarea
                  v-model="editForm.ipBlacklistText"
                  rows="3"
                  class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs"
                  :placeholder="t('adminUsers.ipListPh')"
                />
              </label>
              <label class="flex flex-col gap-1 sm:col-span-2">
                <span class="text-xs text-slate-400">{{ t('adminUsers.lblNewPwd') }}</span>
                <input
                  v-model="editForm.newPassword"
                  type="password"
                  autocomplete="new-password"
                  class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs"
                  :placeholder="t('adminUsers.newPwdPh')"
                />
              </label>
              <label v-if="editForm.totpEnabled" class="flex items-center gap-2 sm:col-span-2">
                <input v-model="editForm.clearTotp" type="checkbox" class="rounded border-slate-600" />
                <span class="text-xs text-slate-400">{{ t('adminUsers.lblClearTotp') }}</span>
              </label>
            </div>
            <div class="mt-6 border-t border-slate-800 pt-5">
              <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
                <h4 class="text-sm font-semibold text-white">{{ t('adminUsers.redeemLedgerTitle') }}</h4>
                <RouterLink
                  v-if="editForm.id"
                  :to="{ path: '/admin/redeem-ledger', query: { user_id: String(editForm.id) } }"
                  class="text-[11px] th-link-accent hover:underline"
                >
                  {{ t('adminUsers.redeemLedgerMore') }}
                  <span v-if="userLedgerTotal > userLedgerRows.length" class="text-slate-500">
                    ({{ userLedgerTotal }})
                  </span>
                </RouterLink>
              </div>
              <p class="mb-2 text-xs text-slate-500">{{ t('adminUsers.redeemLedgerHint') }}</p>
              <p v-if="userLedgerErr" class="text-xs text-rose-400">{{ userLedgerErr }}</p>
              <p v-else-if="!userLedgerRows.length" class="text-xs text-slate-500">{{ t('adminUsers.redeemLedgerEmpty') }}</p>
              <template v-else>
              <div class="hidden overflow-x-auto rounded-lg border border-slate-800 lg:block">
                <table class="w-full text-left text-xs">
                  <thead class="text-slate-500">
                    <tr>
                      <th class="px-2 py-1.5">{{ t('adminUsers.redeemThTime') }}</th>
                      <th class="px-2 py-1.5">{{ t('adminUsers.redeemThHint') }}</th>
                      <th class="px-2 py-1.5">{{ t('adminUsers.redeemThAmount') }}</th>
                      <th class="px-2 py-1.5">{{ t('adminUsers.redeemThBalAfter') }}</th>
                      <th class="px-2 py-1.5">{{ t('adminUsers.redeemThCodeId') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="lr in userLedgerRows" :key="lr.id" class="border-t border-slate-800/80">
                      <td class="px-2 py-1.5 font-mono text-slate-500">{{ lr.created_at || '—' }}</td>
                      <td class="px-2 py-1.5 font-mono text-slate-400">{{ lr.code_hint || '—' }}</td>
                      <td class="px-2 py-1.5 font-mono font-semibold th-redeem-quota-fg">{{ lr.amount }}</td>
                      <td class="px-2 py-1.5 font-mono text-slate-300">{{ lr.balance_after }}</td>
                      <td class="px-2 py-1.5 font-mono text-slate-500">{{ lr.redeem_code_id }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div class="space-y-2 lg:hidden">
                <MobileLabelValueCard v-for="lr in userLedgerRows" :key="`ul-m-${lr.id}`">
                  <template #title>
                    <p class="font-mono text-xs text-slate-500">{{ lr.created_at || '—' }}</p>
                  </template>
                  <MobileLabelValueRow :label="t('adminUsers.redeemThHint')">
                    <span class="font-mono">{{ lr.code_hint || '—' }}</span>
                  </MobileLabelValueRow>
                  <MobileLabelValueRow :label="t('adminUsers.redeemThAmount')">
                    <span class="font-mono font-semibold th-redeem-quota-fg">{{ lr.amount }}</span>
                  </MobileLabelValueRow>
                  <MobileLabelValueRow :label="t('adminUsers.redeemThBalAfter')">
                    <span class="font-mono">{{ lr.balance_after }}</span>
                  </MobileLabelValueRow>
                  <MobileLabelValueRow :label="t('adminUsers.redeemThCodeId')">
                    <span class="font-mono text-slate-500">{{ lr.redeem_code_id }}</span>
                  </MobileLabelValueRow>
                </MobileLabelValueCard>
              </div>
              </template>
            </div>
            <div class="mt-6 flex justify-end gap-2">
              <button type="button" class="rounded-lg px-4 py-2 text-sm text-slate-400 hover:bg-slate-800" :disabled="editSaving" @click="closeEdit">
                {{ t('adminCommon.cancel') }}
              </button>
              <button
                type="button"
                class="th-btn-primary-flat px-4 py-2 text-sm disabled:opacity-50"
                :disabled="editSaving"
                @click="saveEdit"
              >
                {{ t('adminCommon.save') }}
              </button>
            </div>
          </template>
        </div>
      </div>
    </Teleport>
  </div>
</template>
