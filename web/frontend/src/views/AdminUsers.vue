<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { storeToRefs } from 'pinia'
import { http, type Envelope } from '../api/http'
import { useAuthStore } from '../stores/auth'

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

const items = ref<Row[]>([])
const deltaByUser = ref<Record<number, string>>({})
const filterText = ref('')
const compact = ref(false)
const selectedIds = ref<number[]>([])
const flash = ref('')
const err = ref('')

const groups = ref<GroupOpt[]>([])
const editOpen = ref(false)
const editLoading = ref(false)
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

function rowIsElevated(u: Row) {
  return u.role === 'admin' || u.role === 'superadmin'
}

const filteredItems = computed(() => {
  const q = filterText.value.trim().toLowerCase()
  if (!q) return items.value
  return items.value.filter((u) => {
    const idStr = String(u.id)
    const email = (u.email || '').toLowerCase()
    return email.includes(q) || idStr.includes(q)
  })
})

const allFilteredSelected = computed(() => {
  const f = filteredItems.value
  if (!f.length) return false
  return f.every((u) => selectedIds.value.includes(u.id))
})

function toggleSelectAll(checked: boolean) {
  if (checked) {
    const set = new Set(selectedIds.value)
    for (const u of filteredItems.value) set.add(u.id)
    selectedIds.value = [...set]
  } else {
    const drop = new Set(filteredItems.value.map((u) => u.id))
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

async function load() {
  err.value = ''
  try {
    const { data } = await http.get<Envelope<{ items: Row[] }>>('/api/admin/users')
    if (data.code !== 0) throw new Error(data.message || t('adminCommon.loadFail'))
    items.value = data.data.items
    const idSet = new Set(items.value.map((x) => x.id))
    selectedIds.value = selectedIds.value.filter((id) => idSet.has(id))
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('adminCommon.loadFail')
  }
}

function resetFilter() {
  filterText.value = ''
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

async function openEdit(u: Row) {
  err.value = ''
  editOpen.value = true
  editLoading.value = true
  editForm.newPassword = ''
  editForm.clearTotp = false
  try {
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
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('adminCommon.loadFail')
    editOpen.value = false
  } finally {
    editLoading.value = false
  }
}

function closeEdit() {
  editOpen.value = false
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
    await navigator.clipboard.writeText(emails.join('\n'))
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

    <div class="th-console-muted-bar th-console-muted-bar--sm mb-6 flex flex-col gap-3 px-4 py-3 sm:flex-row sm:flex-wrap sm:items-center">
      <input
        v-model="filterText"
        type="search"
        :placeholder="t('adminUsers.searchPh')"
        class="th-input min-w-[12rem] flex-1 text-sm sm:max-w-xs"
      />
      <button type="button" class="th-btn-primary-flat shrink-0 px-3 py-2 text-sm" @click="load">
        {{ t('adminCommon.query') }}
      </button>
      <button type="button" class="th-ghost-hover shrink-0 rounded-lg border border-slate-600 px-3 py-2 text-sm text-slate-200" @click="resetFilter">
        {{ t('adminUsers.resetFilter') }}
      </button>
      <button type="button" class="th-ghost-hover shrink-0 rounded-lg border border-slate-600 px-3 py-2 text-sm text-slate-200" @click="copyEmails">
        {{ t('adminUsers.copyEmails') }}
      </button>
    </div>

    <p v-if="flash" class="mb-3 text-sm text-emerald-400">{{ flash }}</p>
    <p v-if="err" class="mb-4 text-sm text-red-400">{{ err }}</p>

    <div class="th-user-keys-table-wrap overflow-x-auto">
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
          <tr v-for="u in filteredItems" :key="u.id">
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
          <tr v-if="items.length && !filteredItems.length">
            <td colspan="8" class="px-3 py-8 text-center text-slate-500">{{ t('adminUsers.noMatch') }}</td>
          </tr>
          <tr v-if="!items.length">
            <td colspan="8" class="px-3 py-8 text-center text-slate-500">{{ t('adminCommon.dash') }}</td>
          </tr>
        </tbody>
      </table>
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
          <p v-if="editLoading" class="py-8 text-center text-sm text-slate-400">{{ t('adminUsers.editLoading') }}</p>
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
