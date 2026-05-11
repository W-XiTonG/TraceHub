<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'

const { t } = useI18n()

type KeyRow = {
  id: number
  name?: string
  key_prefix?: string
  group_name?: string
  status: string
  last_used_at?: string
  usage_24h?: number
  daily_quota?: number
  ip_whitelist?: string[]
}

type ExpiryPreset = 'never' | '1h' | '1d' | '30d' | 'custom'

const items = ref<KeyRow[]>([])
const err = ref('')
const createdKey = ref('')
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

/** datetime-local 最小可选值（本地） */
const minDatetimeLocal = computed(() => {
  const d = new Date()
  d.setMinutes(d.getMinutes() - d.getTimezoneOffset())
  return d.toISOString().slice(0, 16)
})

async function load() {
  const { data } = await http.get<Envelope<{ items: KeyRow[] }>>('/api/user/apikeys')
  if (data.code === 0) items.value = data.data.items
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
    ? 'border-cyan-500/40 bg-cyan-500/15 text-cyan-100 shadow-[inset_0_1px_0_rgba(255,255,255,0.06)]'
    : 'border-white/[0.06] bg-white/[0.04] text-slate-400 hover:border-white/10 hover:bg-white/[0.06]'
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
    <h1 class="mb-2 text-2xl font-bold">{{ t('apiKeys.title') }}</h1>
    <p class="mb-6 text-sm text-slate-500">{{ t('apiKeys.desc') }}</p>
    <div class="mb-6">
      <button
        type="button"
        class="rounded-lg bg-cyan-500 px-4 py-2 text-sm font-semibold text-slate-950 hover:bg-cyan-400 disabled:opacity-50"
        @click="openCreateModal"
      >
        {{ t('apiKeys.newKey') }}
      </button>
    </div>
    <p v-if="err" class="mb-4 text-sm text-red-400">{{ err }}</p>
    <div v-if="createdKey" class="mb-6 rounded-xl border border-amber-600/50 bg-amber-950/40 p-4 text-sm">
      <p class="font-medium text-amber-400">{{ t('apiKeys.saveOnce') }}</p>
      <code class="mt-2 block break-all text-cyan-300">{{ createdKey }}</code>
    </div>
    <table class="w-full text-left text-sm">
      <thead class="text-slate-500">
        <tr>
          <th class="pb-2">{{ t('apiKeys.thName') }}</th>
          <th class="pb-2">{{ t('apiKeys.thPrefix') }}</th>
          <th class="pb-2">{{ t('apiKeys.thGroup') }}</th>
          <th class="pb-2">{{ t('apiKeys.th24') }}</th>
          <th class="pb-2">{{ t('apiKeys.thStatus') }}</th>
          <th class="pb-2"></th>
        </tr>
      </thead>
      <tbody class="divide-y divide-slate-800">
        <tr v-for="k in items" :key="k.id">
          <td class="py-2">{{ k.name || t('apiKeys.dash') }}</td>
          <td class="py-2 font-mono text-xs">{{ k.key_prefix }}</td>
          <td class="py-2 text-xs text-slate-400">{{ k.group_name || t('apiKeys.dash') }}</td>
          <td class="py-2 text-xs">{{ k.usage_24h ?? t('apiKeys.dash') }}</td>
          <td class="py-2">{{ k.status }}</td>
          <td class="space-x-2 py-2 text-right">
            <button type="button" class="text-cyan-400 hover:underline" @click="openEdit(k)">{{ t('apiKeys.editBtn') }}</button>
            <button type="button" class="text-red-400 hover:underline" @click="remove(k.id)">{{ t('apiKeys.deleteBtn') }}</button>
          </td>
        </tr>
      </tbody>
    </table>

    <Transition name="th-drawer">
      <div
        v-if="createOpen"
        class="fixed inset-0 z-40 flex justify-end"
        role="dialog"
        aria-modal="true"
        aria-labelledby="create-key-drawer-title"
      >
        <div class="absolute inset-0 bg-black/60 backdrop-blur-[2px]" aria-hidden="true" @click="closeCreateModal" />
        <aside
          class="th-drawer-panel relative z-10 flex h-full w-full max-w-md flex-col border-l border-slate-700/90 bg-slate-900 shadow-2xl shadow-black/40"
        >
          <header class="flex shrink-0 items-start justify-between gap-4 border-b border-white/10 px-5 py-4">
            <div class="min-w-0">
              <h2 id="create-key-drawer-title" class="text-lg font-semibold tracking-tight text-slate-100">
                {{ t('apiKeys.createTitle') }}
              </h2>
              <p class="mt-1 text-[13px] leading-relaxed text-slate-500">{{ t('apiKeys.createDrawerSubtitle') }}</p>
            </div>
            <button
              type="button"
              class="shrink-0 rounded-lg p-2 text-slate-400 transition hover:bg-white/10 hover:text-slate-200"
              :aria-label="t('apiKeys.closeDrawer')"
              @click="closeCreateModal"
            >
              <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </header>

          <div class="min-h-0 flex-1 overflow-y-auto px-5 py-5">
            <div class="space-y-5">
              <section class="rounded-xl border border-white/[0.06] bg-black/25 p-4">
                <div class="mb-4 flex gap-3">
                  <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-sky-500/15 text-sky-300 ring-1 ring-sky-500/20">
                    <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M15.75 5.25a3 3 0 013 3m3 0a6 6 0 01-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H3.75v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1121.75 8.25z"
                      />
                    </svg>
                  </span>
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
                    class="mt-1.5 w-full rounded-lg border border-slate-600/80 bg-slate-950/80 px-3 py-2.5 text-sm text-slate-100 placeholder:text-slate-600 focus:border-cyan-500/50 focus:outline-none focus:ring-2 focus:ring-cyan-500/20"
                    @keydown.enter.prevent="submitCreateKey"
                  />
                </label>

                <div class="mt-4 rounded-lg border border-white/[0.05] bg-white/[0.02] px-3 py-2.5">
                  <p class="text-[11px] font-medium uppercase tracking-wide text-slate-500">{{ t('apiKeys.routingGroup') }}</p>
                  <p v-if="myGroupName" class="mt-1 text-[13px] leading-relaxed text-slate-400">
                    {{ t('apiKeys.routingGroupHint', { name: myGroupName }) }}
                  </p>
                  <p v-else class="mt-1 text-[13px] leading-relaxed text-amber-200/85">{{ t('apiKeys.routingGroupUnknown') }}</p>
                </div>

                <div class="mt-4">
                  <p class="mb-2 text-[13px] text-slate-400">{{ t('apiKeys.expiryLabel') }}</p>
                  <div class="flex flex-wrap gap-2">
                    <button type="button" :class="['rounded-lg border px-3 py-1.5 text-xs font-medium transition', chipCls(expiryPreset === 'never')]" @click="expiryPreset = 'never'">
                      {{ t('apiKeys.expiryNever') }}
                    </button>
                    <button type="button" :class="['rounded-lg border px-3 py-1.5 text-xs font-medium transition', chipCls(expiryPreset === '1h')]" @click="expiryPreset = '1h'">
                      {{ t('apiKeys.expiry1h') }}
                    </button>
                    <button type="button" :class="['rounded-lg border px-3 py-1.5 text-xs font-medium transition', chipCls(expiryPreset === '1d')]" @click="expiryPreset = '1d'">
                      {{ t('apiKeys.expiry1d') }}
                    </button>
                    <button type="button" :class="['rounded-lg border px-3 py-1.5 text-xs font-medium transition', chipCls(expiryPreset === '30d')]" @click="expiryPreset = '30d'">
                      {{ t('apiKeys.expiry30d') }}
                    </button>
                    <button type="button" :class="['rounded-lg border px-3 py-1.5 text-xs font-medium transition', chipCls(expiryPreset === 'custom')]" @click="expiryPreset = 'custom'">
                      {{ t('apiKeys.expiryCustom') }}
                    </button>
                  </div>
                  <div v-if="expiryPreset === 'custom'" class="mt-3">
                    <input
                      v-model="expiryCustom"
                      type="datetime-local"
                      :min="minDatetimeLocal"
                      class="w-full rounded-lg border border-slate-600/80 bg-slate-950/80 px-3 py-2 text-sm text-slate-100 focus:border-cyan-500/50 focus:outline-none focus:ring-2 focus:ring-cyan-500/20"
                    />
                    <p class="mt-1.5 text-[11px] text-slate-600">{{ t('apiKeys.expiryCustomHint') }}</p>
                  </div>
                </div>
              </section>

              <section class="rounded-xl border border-white/[0.06] bg-black/25 p-4">
                <div class="mb-4 flex gap-3">
                  <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-emerald-500/15 text-emerald-300 ring-1 ring-emerald-500/20">
                    <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M21 12a2.25 2.25 0 00-2.25-2.25H15a3 3 0 11-6 0H5.25A2.25 2.25 0 003 12m18 0v6a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 18v-6m18 0V9M3 12V9m18 3a2.25 2.25 0 01-2.25-2.25H15a3 3 0 10-6 0H5.25A2.25 2.25 0 003 12"
                      />
                    </svg>
                  </span>
                  <div class="min-w-0">
                    <h3 class="text-sm font-semibold text-slate-100">{{ t('apiKeys.sectionQuota') }}</h3>
                    <p class="mt-0.5 text-xs leading-relaxed text-slate-500">{{ t('apiKeys.sectionQuotaDesc') }}</p>
                  </div>
                </div>

                <label class="flex cursor-pointer items-start gap-3 rounded-lg border border-transparent py-1 transition hover:border-white/[0.06]">
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
                      class="mt-1.5 w-full rounded-lg border border-slate-600/80 bg-slate-950/80 px-3 py-2.5 font-mono text-sm text-slate-100 focus:border-cyan-500/50 focus:outline-none focus:ring-2 focus:ring-cyan-500/20"
                    />
                  </label>
                </div>
              </section>

              <section class="rounded-xl border border-white/[0.06] bg-black/25 p-4">
                <div class="mb-3 flex gap-3">
                  <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-violet-500/15 text-violet-300 ring-1 ring-violet-500/20">
                    <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M2.036 12.322a1.012 1.012 0 010-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178z" />
                      <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                    </svg>
                  </span>
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

          <footer class="flex shrink-0 justify-end gap-2 border-t border-white/10 bg-slate-900/95 px-5 py-4">
            <button type="button" class="rounded-lg border border-slate-600 px-4 py-2 text-sm text-slate-300 hover:bg-white/5" @click="closeCreateModal">
              {{ t('apiKeys.cancel') }}
            </button>
            <button
              type="button"
              :disabled="loading"
              class="inline-flex items-center gap-2 rounded-lg bg-cyan-600 px-4 py-2 text-sm font-medium text-white hover:bg-cyan-500 disabled:opacity-50"
              @click="submitCreateKey"
            >
              <svg class="h-4 w-4 opacity-90" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z" />
              </svg>
              {{ loading ? t('apiKeys.creating') : t('apiKeys.submitCreate') }}
            </button>
          </footer>
        </aside>
      </div>
    </Transition>

    <Transition name="th-modal">
      <div v-if="editOpen" class="fixed inset-0 z-40 flex items-center justify-center p-4">
        <div class="absolute inset-0 bg-black/60" aria-hidden="true" @click="editOpen = false" />
        <div class="th-modal-surface relative z-10 w-full max-w-md rounded-xl border border-slate-700 bg-slate-900 p-6 shadow-xl">
          <h3 class="mb-4 font-semibold">{{ t('apiKeys.editTitle') }}</h3>
          <label class="mb-3 block text-sm">
            <span class="text-slate-500">{{ t('apiKeys.labelName') }}</span>
            <input v-model="edit.name" class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-2 py-1.5" />
          </label>
          <label class="mb-3 flex items-center gap-2 text-sm">
            <input v-model="edit.enabled" type="checkbox" /> {{ t('apiKeys.enabled') }}
          </label>
          <label class="mb-3 block text-sm">
            <span class="text-slate-500">{{ t('apiKeys.dailyQuota') }}</span>
            <input v-model="edit.daily_quota" :placeholder="t('apiKeys.dailyQuotaPh')" class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-2 py-1.5 font-mono text-xs" />
          </label>
          <label class="mb-4 block text-sm">
            <span class="text-slate-500">{{ t('apiKeys.ipWhitelist') }}</span>
            <textarea v-model="edit.ip_whitelist" rows="3" class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-2 py-1.5 font-mono text-xs" />
          </label>
          <div class="flex justify-end gap-2">
            <button type="button" class="rounded border border-slate-600 px-3 py-1.5 text-sm" @click="editOpen = false">{{ t('apiKeys.cancel') }}</button>
            <button type="button" class="rounded bg-cyan-600 px-3 py-1.5 text-sm font-medium text-white" @click="saveEdit">{{ t('apiKeys.save') }}</button>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>
