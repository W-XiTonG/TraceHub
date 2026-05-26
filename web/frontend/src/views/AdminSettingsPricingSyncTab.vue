<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'

const { t } = useI18n()

const DEFAULT_PULL = '/api/pricing/sync/export'

type TabId = 'start' | 'server' | 'client' | 'advanced'

type Target = {
  id: number
  name: string
  base_url: string
  pull_path: string
  has_static_secret?: boolean
  enabled: boolean
  scopes: string[]
  model_include_mode: string
  model_allowlist: string[]
  model_denylist: string[]
  merge_strategy: string
  schedule_enabled: boolean
  schedule_interval_min: number
  last_pull_at?: string
  last_pull_ok?: boolean
  last_pull_message?: string
}

type InboundInfo = {
  configured: boolean
  token_created_at?: string
  export_url: string
  export_path: string
  default_pull_path: string
}

type DiffRow = {
  layer: string
  model_name: string
  channel_id?: number
  group_id?: number
  change: string
  selected: boolean
  detail?: string
  field_changes?: FieldChange[]
}

type FieldChange = {
  label: string
  old?: string
  new: string
}

type PreviewResult = {
  preview_id: string
  expires_at: string
  added: number
  updated: number
  skipped: number
  warnings: string[]
  rows: DiffRow[]
}

type TestResult = { ok: boolean; message?: string }

const activeTab = ref<TabId>('start')
const targets = ref<Target[]>([])
const featureEnabled = ref(false)
const inbound = ref<InboundInfo | null>(null)
const msg = ref('')
const err = ref('')
const { busy, busyUi, run } = useSmoothLoading()

const drawerOpen = ref(false)
const editingId = ref<number | null>(null)
const formSaving = ref(false)

const form = reactive({
  name: '',
  base_url: '',
  pull_path: DEFAULT_PULL,
  api_key: '',
  has_static_secret: false,
  scopes: ['global', 'channel', 'group'] as string[],
  model_include_mode: 'all',
  model_allowlist_text: '',
  model_denylist_text: '',
  merge_strategy: 'upsert',
  schedule_enabled: false,
  schedule_interval_min: 60,
  enabled: true,
})

const showFormAdvanced = ref(false)
const inboundTokenPlain = ref('')
const regeneratingInbound = ref(false)
const generatingOutbound = ref(false)
const testingTargetId = ref<number | null>(null)

const syncModalOpen = ref(false)
const syncTarget = ref<Target | null>(null)
const preview = ref<PreviewResult | null>(null)
const previewStale = ref(false)
const previewLoading = ref(false)
let previewReqSeq = 0
const confirmChecked = ref(false)
const syncing = ref(false)

const featureSwitch = ref(false)
const workerEnabled = ref(false)
const savingSettings = ref(false)

const scopeOptions = ['global', 'channel', 'group']
const isEditing = computed(() => editingId.value != null)
const selectedRowCount = computed(() => preview.value?.rows.filter((r) => r.selected).length ?? 0)

const syncModalTitle = computed(() =>
  t('adminPricingSync.syncModalPull', { name: syncTarget.value?.name ?? '' }),
)

const canConfirmSync = computed(
  () => confirmChecked.value && preview.value && !previewStale.value && !syncing.value,
)

function parseLines(raw: string): string[] {
  return raw.split(/[\n,，;；]+/).map((s) => s.trim()).filter(Boolean)
}

function layerLabel(layer: string) {
  if (layer === 'channel') return t('adminPricingSync.layerChannel')
  if (layer === 'group') return t('adminPricingSync.layerGroup')
  return t('adminPricingSync.layerGlobal')
}

function changeLabel(change: string) {
  const m: Record<string, string> = {
    added: 'adminPricingSync.changeAdded',
    updated: 'adminPricingSync.changeUpdated',
    skipped: 'adminPricingSync.changeSkipped',
    removed: 'adminPricingSync.changeRemoved',
  }
  return m[change] ? t(m[change]) : change
}

function lastSyncSummary(tgt: Target): string {
  if (!tgt.last_pull_at) return '—'
  return `${tgt.last_pull_ok ? 'OK' : 'FAIL'}`
}

function lastSyncError(tgt: Target): string {
  if (tgt.last_pull_at && !tgt.last_pull_ok && tgt.last_pull_message) return tgt.last_pull_message
  return ''
}

function resetForm() {
  editingId.value = null
  form.name = ''
  form.base_url = ''
  form.pull_path = inbound.value?.default_pull_path || DEFAULT_PULL
  form.api_key = ''
  form.has_static_secret = false
  form.scopes = ['global', 'channel', 'group']
  form.model_include_mode = 'all'
  form.model_allowlist_text = ''
  form.model_denylist_text = ''
  form.merge_strategy = 'upsert'
  form.schedule_enabled = false
  form.schedule_interval_min = 60
  form.enabled = true
  showFormAdvanced.value = false
}

function openCreateDrawer() {
  resetForm()
  drawerOpen.value = true
}

function openEditDrawer(tgt: Target) {
  editingId.value = tgt.id
  form.name = tgt.name
  form.base_url = tgt.base_url
  form.pull_path = tgt.pull_path || DEFAULT_PULL
  form.api_key = ''
  form.has_static_secret = !!tgt.has_static_secret
  form.scopes = [...(tgt.scopes || ['global', 'channel', 'group'])]
  form.model_include_mode = tgt.model_include_mode || 'all'
  form.model_allowlist_text = (tgt.model_allowlist || []).join('\n')
  form.model_denylist_text = (tgt.model_denylist || []).join('\n')
  form.merge_strategy = tgt.merge_strategy || 'upsert'
  form.schedule_enabled = !!tgt.schedule_enabled
  form.schedule_interval_min = tgt.schedule_interval_min > 0 ? tgt.schedule_interval_min : 60
  form.enabled = tgt.enabled
  drawerOpen.value = true
}

function goClientTab() {
  activeTab.value = 'client'
  openCreateDrawer()
}

async function loadInbound() {
  const { data } = await http.get<Envelope<InboundInfo & { import_url?: string }>>(
    '/api/admin/pricing-sync/inbound-info',
  )
  if (data.code === 0) {
    inbound.value = {
      configured: data.data.configured,
      token_created_at: data.data.token_created_at,
      export_url: data.data.export_url,
      export_path: data.data.export_path,
      default_pull_path: data.data.default_pull_path || DEFAULT_PULL,
    }
  }
}

async function loadAll() {
  await run(async () => {
    err.value = ''
    const { data } = await http.get<
      Envelope<{ items: Target[]; feature_enabled: boolean; settings?: Record<string, unknown> }>
    >('/api/admin/pricing-sync/targets')
    if (data.code === 0) {
      targets.value = data.data.items
      featureEnabled.value = !!data.data.feature_enabled
      featureSwitch.value = featureEnabled.value
      const s = data.data.settings ?? {}
      workerEnabled.value = s['site.pricing_sync.worker_enabled_db_value'] === 'true'
    }
    await loadInbound()
  })
}

async function saveTarget() {
  formSaving.value = true
  err.value = ''
  const body = {
    name: form.name,
    direction: 'pull',
    base_url: form.base_url,
    pull_path: form.pull_path,
    api_key: form.api_key || undefined,
    scopes: form.scopes,
    model_include_mode: form.model_include_mode,
    model_allowlist: parseLines(form.model_allowlist_text),
    model_denylist: parseLines(form.model_denylist_text),
    merge_strategy: form.merge_strategy,
    schedule_enabled: form.schedule_enabled,
    schedule_interval_min: form.schedule_enabled ? form.schedule_interval_min : undefined,
    enabled: form.enabled,
  }
  try {
    if (isEditing.value) {
      await http.put(`/api/admin/pricing-sync/targets/${editingId.value}`, body)
    } else {
      await http.post('/api/admin/pricing-sync/targets', body)
    }
    msg.value = t('adminPricingSync.saved')
    drawerOpen.value = false
    resetForm()
    await loadAll()
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('adminPricingSync.saveFail')
  } finally {
    formSaving.value = false
  }
}

async function toggleTargetEnabled(tgt: Target) {
  try {
    await http.put(`/api/admin/pricing-sync/targets/${tgt.id}`, { enabled: !tgt.enabled, direction: 'pull' })
    await loadAll()
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('adminPricingSync.saveFail')
  }
}

async function testConnection(tgt: Target) {
  testingTargetId.value = tgt.id
  err.value = ''
  msg.value = ''
  try {
    const { data } = await http.post<Envelope<TestResult>>(
      `/api/admin/pricing-sync/targets/${tgt.id}/test-connection`,
      { direction: 'pull' },
    )
    if (data.code === 0) {
      msg.value = data.data.ok
        ? `${t('adminPricingSync.testOk')}: ${data.data.message ?? ''}`
        : `${t('adminPricingSync.testFail')}: ${data.data.message ?? ''}`
    } else {
      err.value = data.message ?? t('adminPricingSync.testFail')
    }
  } catch (e: unknown) {
    if (e && typeof e === 'object' && 'response' in e) {
      const resp = (e as { response?: { data?: { message?: string } } }).response
      err.value = resp?.data?.message ?? t('adminPricingSync.testFail')
    } else {
      err.value = e instanceof Error ? e.message : t('adminPricingSync.testFail')
    }
  } finally {
    testingTargetId.value = null
  }
}

async function deleteTarget(id: number) {
  if (!confirm(t('adminPricingSync.confirmDelete'))) return
  await http.delete(`/api/admin/pricing-sync/targets/${id}`)
  await loadAll()
}

function toggleScope(s: string) {
  const i = form.scopes.indexOf(s)
  if (i >= 0) form.scopes.splice(i, 1)
  else form.scopes.push(s)
}

async function regenerateInboundToken() {
  if (!confirm(t('adminPricingSync.inboundRegenerate') + '?')) return
  regeneratingInbound.value = true
  err.value = ''
  try {
    const { data } = await http.post<
      Envelope<{ token: string; export_url: string; message?: string }>
    >('/api/admin/pricing-sync/inbound-token/regenerate')
    if (data.code === 0) {
      inboundTokenPlain.value = data.data.token
      if (inbound.value) {
        inbound.value.configured = true
        inbound.value.export_url = data.data.export_url
      }
      msg.value = data.data.message ?? t('adminPricingSync.saved')
    }
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('adminPricingSync.saveFail')
  } finally {
    regeneratingInbound.value = false
  }
}

async function generateOutboundKey() {
  generatingOutbound.value = true
  try {
    const { data } = await http.post<Envelope<{ api_key: string }>>(
      '/api/admin/pricing-sync/generate-outbound-key',
    )
    if (data.code === 0) form.api_key = data.data.api_key
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('adminPricingSync.saveFail')
  } finally {
    generatingOutbound.value = false
  }
}

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    msg.value = t('adminPricingSync.copyOk')
  } catch {
    err.value = t('adminPricingSync.saveFail')
  }
}

function copyClientBundle() {
  if (!inbound.value) return
  const exportUrl = inbound.value.export_url || inbound.value.export_path
  const token = inboundTokenPlain.value
  const lines = [
    t('adminPricingSync.copyClientBundleHeader'),
    `${t('adminPricingSync.serverBaseUrl')}: ${exportUrl.replace(/\/api\/pricing\/sync\/export.*$/, '') || '(your site root)'}`,
    `${t('adminPricingSync.pullPath')}: ${DEFAULT_PULL}`,
    token ? `${t('adminPricingSync.inboundTokenStatus')}: ${token}` : t('adminPricingSync.inboundNotConfigured'),
    t('adminPricingSync.copyClientBundleAuth'),
  ]
  void copyText(lines.join('\n'))
}

function openSyncModal(tgt: Target) {
  syncTarget.value = tgt
  preview.value = null
  previewStale.value = false
  confirmChecked.value = false
  syncModalOpen.value = true
  void loadPreview()
}

async function loadPreview() {
  if (!syncTarget.value) return
  const seq = ++previewReqSeq
  previewLoading.value = true
  err.value = ''
  try {
    const selectedKeys = preview.value?.rows.filter((r) => r.selected).map(rowKey) ?? []
    const { data } = await http.post<Envelope<PreviewResult>>(
      `/api/admin/pricing-sync/targets/${syncTarget.value.id}/sync/preview`,
      { direction: 'pull', selected_keys: selectedKeys.length ? selectedKeys : undefined },
    )
    if (seq !== previewReqSeq) return
    if (data.code === 0) {
      preview.value = data.data
      previewStale.value = false
    }
  } catch (e: unknown) {
    if (seq !== previewReqSeq) return
    err.value = e instanceof Error ? e.message : t('adminPricingSync.previewFail')
  } finally {
    if (seq === previewReqSeq) previewLoading.value = false
  }
}

function rowKey(r: DiffRow) {
  if (r.layer === 'channel') return `channel:${r.channel_id ?? 0}:${r.model_name}`
  if (r.layer === 'group') return `group:${r.group_id ?? 0}:${r.model_name}`
  return `global:${r.model_name}`
}

function toggleRow(r: DiffRow) {
  if (!preview.value) return
  const k = rowKey(r)
  preview.value.rows = preview.value.rows.map((x) =>
    rowKey(x) === k ? { ...x, selected: !x.selected } : x,
  )
  previewStale.value = true
}

function selectAllRows(on: boolean) {
  if (!preview.value) return
  preview.value.rows = preview.value.rows.map((x) => ({
    ...x,
    selected: on && (x.change === 'added' || x.change === 'updated'),
  }))
  previewStale.value = true
}

async function confirmSync() {
  if (!syncTarget.value || !preview.value || !canConfirmSync.value) return
  syncing.value = true
  err.value = ''
  try {
    const { data } = await http.post<Envelope<{ message: string }>>(
      `/api/admin/pricing-sync/targets/${syncTarget.value.id}/sync`,
      { preview_id: preview.value.preview_id, direction: 'pull' },
    )
    if (data.code === 0) {
      msg.value = data.data?.message ?? t('adminPricingSync.syncDone')
      syncModalOpen.value = false
      await loadAll()
    }
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('adminPricingSync.syncFail')
  } finally {
    syncing.value = false
  }
}

async function saveAdvancedSettings() {
  savingSettings.value = true
  try {
    await http.put('/api/admin/config', {
      items: [
        { key: 'site.pricing_sync.enabled', value: featureSwitch.value ? 'true' : 'false' },
        { key: 'site.pricing_sync.worker_enabled', value: workerEnabled.value ? 'true' : 'false' },
      ],
    })
    featureEnabled.value = featureSwitch.value
    msg.value = t('adminPricingSync.settingsSaved')
    await loadAll()
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('adminPricingSync.saveFail')
  } finally {
    savingSettings.value = false
  }
}

watch(drawerOpen, (open) => {
  if (!open) resetForm()
})

onMounted(() => void loadAll())
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap gap-2 text-xs">
      <span
        class="rounded-full px-2.5 py-1"
        :class="featureEnabled ? 'bg-emerald-500/20 text-emerald-300' : 'bg-amber-500/20 text-amber-200'"
      >
        {{ featureEnabled ? t('adminPricingSync.statusFeatureOn') : t('adminPricingSync.statusFeatureOff') }}
      </span>
      <span
        class="rounded-full px-2.5 py-1"
        :class="inbound?.configured ? 'bg-emerald-500/20 text-emerald-300' : 'bg-slate-700 text-slate-400'"
      >
        {{ inbound?.configured ? t('adminPricingSync.statusInboundOk') : t('adminPricingSync.statusInboundMissing') }}
      </span>
      <span class="rounded-full bg-slate-700 px-2.5 py-1 text-slate-300">
        {{ t('adminPricingSync.statusTargets', { n: targets.length }) }}
      </span>
    </div>

    <p v-if="msg" class="text-sm text-emerald-400">{{ msg }}</p>
    <p v-if="err" class="text-sm text-red-400">{{ err }}</p>

    <nav class="flex flex-wrap gap-1 border-b border-slate-700/60 pb-2">
      <button
        v-for="tab in (['start', 'server', 'client', 'advanced'] as TabId[])"
        :key="tab"
        type="button"
        class="rounded-lg px-3 py-1.5 text-sm font-medium transition-colors"
        :class="activeTab === tab ? 'bg-sky-600/30 text-sky-200' : 'text-slate-400 hover:bg-slate-800 hover:text-slate-200'"
        @click="activeTab = tab"
      >
        {{
          tab === 'start'
            ? t('adminPricingSync.tabStart')
            : tab === 'server'
              ? t('adminPricingSync.tabServer')
              : tab === 'client'
                ? t('adminPricingSync.tabClient')
                : t('adminPricingSync.tabAdvanced')
        }}
      </button>
    </nav>

    <ThLoadingState v-if="busyUi" />

    <!-- 快速开始 -->
    <div v-else-if="activeTab === 'start'" class="space-y-4">
      <p v-if="!featureEnabled" class="rounded-lg border border-amber-500/30 bg-amber-500/10 px-4 py-3 text-sm text-amber-100/90">
        {{ t('adminPricingSync.featureDisabledHint') }}
        <button type="button" class="ml-2 underline" @click="activeTab = 'advanced'">{{ t('adminPricingSync.enableFeature') }}</button>
      </p>
      <p class="text-xs text-slate-400">{{ t('adminPricingSync.pullOnlyHint') }}</p>
      <div class="grid gap-4 md:grid-cols-2">
        <article class="rounded-xl border border-slate-700/60 bg-slate-900/40 p-4">
          <h3 class="text-sm font-semibold text-slate-200">{{ t('adminPricingSync.roleServerTitle') }}</h3>
          <p class="mt-2 text-xs text-slate-400">{{ t('adminPricingSync.roleServerIntro') }}</p>
          <p class="mt-3 text-xs leading-relaxed text-slate-500">{{ t('adminPricingSync.roleServerSteps') }}</p>
          <button type="button" class="th-btn-secondary mt-4 px-3 py-1.5 text-xs" @click="activeTab = 'server'">
            {{ t('adminPricingSync.scenarioGoInbound') }}
          </button>
        </article>
        <article class="rounded-xl border border-slate-700/60 bg-slate-900/40 p-4">
          <h3 class="text-sm font-semibold text-slate-200">{{ t('adminPricingSync.roleClientTitle') }}</h3>
          <p class="mt-2 text-xs text-slate-400">{{ t('adminPricingSync.roleClientIntro') }}</p>
          <p class="mt-3 text-xs leading-relaxed text-slate-500">{{ t('adminPricingSync.roleClientSteps') }}</p>
          <button type="button" class="th-btn-secondary mt-4 px-3 py-1.5 text-xs" @click="goClientTab">
            {{ t('adminPricingSync.scenarioGoOutbound') }}
          </button>
        </article>
      </div>
    </div>

    <!-- 服务端 -->
    <section v-else-if="activeTab === 'server'" class="rounded-xl border border-slate-700/60 bg-slate-900/40 p-4">
      <h2 class="text-sm font-semibold text-slate-200">{{ t('adminPricingSync.tabServer') }}</h2>
      <p class="mt-2 text-xs text-slate-400">{{ t('adminPricingSync.roleServerIntro') }}</p>
      <template v-if="inbound">
        <div class="mt-4 space-y-3 text-sm">
          <div>
            <span class="text-xs text-slate-500">{{ t('adminPricingSync.inboundExportUrl') }}</span>
            <code class="mt-1 block break-all rounded bg-slate-800/80 px-2 py-1 text-xs">{{ inbound.export_url || inbound.export_path }}</code>
          </div>
          <p class="text-xs text-slate-500">
            {{ t('adminPricingSync.inboundTokenStatus') }}：
            <span :class="inbound.configured ? 'text-emerald-400' : 'text-amber-400'">
              {{ inbound.configured ? t('adminPricingSync.inboundConfigured') : t('adminPricingSync.inboundNotConfigured') }}
            </span>
          </p>
        </div>
        <div v-if="inboundTokenPlain" class="mt-3 rounded-lg border border-amber-500/40 bg-amber-500/10 p-3">
          <p class="text-xs text-amber-200">{{ t('adminPricingSync.inboundTokenOnce') }}</p>
          <code class="mt-2 block break-all font-mono text-sm text-amber-100">{{ inboundTokenPlain }}</code>
        </div>
        <div class="mt-4 flex flex-wrap gap-2">
          <button
            type="button"
            class="th-btn-primary px-4 py-2 text-sm"
            :disabled="!featureEnabled || regeneratingInbound"
            @click="regenerateInboundToken"
          >
            {{ regeneratingInbound ? t('common.submitting') : t('adminPricingSync.inboundRegenerate') }}
          </button>
          <button type="button" class="th-btn-secondary px-4 py-2 text-sm" @click="copyClientBundle">
            {{ t('adminPricingSync.copyClientBundle') }}
          </button>
        </div>
        <p v-if="!featureEnabled" class="mt-3 text-xs text-amber-400">{{ t('adminPricingSync.featureDisabledHint') }}</p>
      </template>
    </section>

    <!-- 客户端 -->
    <section v-else-if="activeTab === 'client'" class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <p class="text-xs text-slate-400">{{ t('adminPricingSync.roleClientIntro') }}</p>
        <button type="button" class="th-btn-primary px-4 py-2 text-sm" @click="openCreateDrawer">
          {{ t('adminPricingSync.addTarget') }}
        </button>
      </div>
      <p v-if="!featureEnabled" class="text-xs text-amber-400">
        {{ t('adminPricingSync.featureDisabledHint') }}
        {{ t('adminPricingSync.testWithoutFeatureHint') }}
      </p>
      <p v-if="targets.length === 0" class="text-sm text-slate-500">{{ t('adminPricingSync.noTargets') }}</p>
      <div v-else class="overflow-hidden rounded-xl border border-slate-700/60">
        <table class="w-full text-left text-sm">
          <thead class="bg-slate-800/80 text-xs text-slate-400">
            <tr>
              <th class="px-3 py-2">{{ t('adminPricingSync.name') }}</th>
              <th class="px-3 py-2">{{ t('adminPricingSync.serverBaseUrl') }}</th>
              <th class="px-3 py-2">{{ t('adminPricingSync.enabled') }}</th>
              <th class="px-3 py-2">{{ t('adminPricingSync.lastSync') }}</th>
              <th class="px-3 py-2">{{ t('adminPricingSync.colActions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="tgt in targets" :key="tgt.id" class="border-t border-slate-700/50">
              <td class="px-3 py-2 font-medium">{{ tgt.name }}</td>
              <td class="max-w-[200px] truncate px-3 py-2 font-mono text-xs" :title="tgt.base_url">{{ tgt.base_url }}</td>
              <td class="px-3 py-2">
                <input type="checkbox" :checked="tgt.enabled" @change="toggleTargetEnabled(tgt)" />
              </td>
              <td class="px-3 py-2 text-xs" :title="lastSyncError(tgt)">
                <span :class="lastSyncError(tgt) ? 'text-red-400' : 'text-slate-400'">{{ lastSyncSummary(tgt) }}</span>
              </td>
              <td class="px-3 py-2">
                <div class="flex flex-wrap gap-x-2 gap-y-1 text-xs">
                  <button type="button" class="text-slate-300 hover:underline" @click="openEditDrawer(tgt)">
                    {{ t('adminPricingSync.editTarget') }}
                  </button>
                  <button
                    type="button"
                    class="text-violet-400 hover:underline"
                    :disabled="testingTargetId === tgt.id"
                    @click="testConnection(tgt)"
                  >
                    {{ t('adminPricingSync.testConnection') }}
                  </button>
                  <button
                    type="button"
                    class="text-sky-400 hover:underline"
                    :disabled="!featureEnabled"
                    @click="openSyncModal(tgt)"
                  >
                    {{ t('adminPricingSync.actionPull') }}
                  </button>
                  <button type="button" class="text-red-400 hover:underline" @click="deleteTarget(tgt.id)">
                    {{ t('adminCommon.delete') }}
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <!-- 高级 -->
    <section v-else-if="activeTab === 'advanced'" class="space-y-4 rounded-xl border border-slate-700/60 bg-slate-900/40 p-4">
      <div>
        <label class="flex items-center gap-2 text-sm text-slate-300">
          <input v-model="featureSwitch" type="checkbox" class="rounded" />
          {{ t('adminPricingSync.featureSwitch') }}
        </label>
        <p class="mt-1 text-xs text-slate-500">{{ t('adminPricingSync.featureSwitchHint') }}</p>
      </div>
      <div>
        <label class="flex items-center gap-2 text-sm text-slate-300">
          <input v-model="workerEnabled" type="checkbox" class="rounded" />
          {{ t('adminPricingSync.workerEnabled') }}
        </label>
        <p class="mt-1 text-xs text-slate-500">{{ t('adminPricingSync.workerEnabledHint') }}</p>
      </div>
      <button type="button" class="th-btn-primary px-4 py-2 text-sm" :disabled="savingSettings" @click="saveAdvancedSettings">
        {{ savingSettings ? t('common.submitting') : t('adminCommon.save') }}
      </button>
    </section>

    <!-- 抽屉 -->
    <div v-if="drawerOpen" class="fixed inset-0 z-40 flex justify-end bg-black/50" @click.self="drawerOpen = false">
      <div class="h-full w-full max-w-lg overflow-y-auto border-l border-slate-600 bg-slate-900 p-5 shadow-xl">
        <h3 class="text-lg font-semibold">{{ isEditing ? t('adminPricingSync.editTarget') : t('adminPricingSync.addTarget') }}</h3>
        <div class="mt-4 grid gap-3">
          <label class="block text-xs text-slate-500">
            {{ t('adminPricingSync.name') }}
            <input v-model="form.name" class="th-input mt-1 w-full" />
          </label>
          <label class="block text-xs text-slate-500">
            {{ t('adminPricingSync.serverBaseUrl') }}
            <input v-model="form.base_url" class="th-input mt-1 w-full" placeholder="https://peer.example.com" />
          </label>
          <label class="block text-xs text-slate-500">
            {{ t('adminPricingSync.pullPath') }}
            <input v-model="form.pull_path" class="th-input mt-1 w-full font-mono text-xs" />
          </label>
          <div>
            <label class="block text-xs text-slate-500">
              {{ t('adminPricingSync.apiKey') }}
              <div class="mt-1 flex gap-2">
                <input
                  v-model="form.api_key"
                  type="password"
                  class="th-input flex-1"
                  :placeholder="form.has_static_secret ? t('adminPricingSync.apiKeyKeepHint') : ''"
                  autocomplete="new-password"
                />
                <button type="button" class="th-btn-secondary shrink-0 px-3 py-2 text-xs" :disabled="generatingOutbound" @click="generateOutboundKey">
                  {{ t('adminPricingSync.generateOutboundKey') }}
                </button>
              </div>
            </label>
            <p class="mt-1 text-xs text-slate-600">{{ t('adminPricingSync.traceHubKeyHint') }}</p>
          </div>
          <button type="button" class="text-xs text-slate-400 underline" @click="showFormAdvanced = !showFormAdvanced">
            {{ t('adminPricingSync.advancedFilters') }}
          </button>
          <div v-show="showFormAdvanced" class="grid gap-3 border-t border-slate-700/50 pt-3">
            <label class="block text-xs text-slate-500">
              merge_strategy
              <select v-model="form.merge_strategy" class="th-input mt-1 w-full">
                <option value="upsert">{{ t('adminPricingSync.mergeUpsert') }}</option>
                <option value="skip_existing">{{ t('adminPricingSync.mergeSkip') }}</option>
                <option value="replace_scoped">{{ t('adminPricingSync.mergeReplace') }}</option>
              </select>
            </label>
            <div class="flex flex-wrap gap-3">
              <span class="text-xs text-slate-500">{{ t('adminPricingSync.scopes') }}</span>
              <label v-for="s in scopeOptions" :key="s" class="flex items-center gap-1 text-sm">
                <input type="checkbox" :checked="form.scopes.includes(s)" @change="toggleScope(s)" />
                {{ s }}
              </label>
            </div>
            <label class="flex items-center gap-2 text-sm">
              <input v-model="form.schedule_enabled" type="checkbox" class="rounded" />
              {{ t('adminPricingSync.scheduleEnabled') }}
            </label>
            <label v-if="form.schedule_enabled" class="block text-xs text-slate-500">
              {{ t('adminPricingSync.scheduleIntervalMin') }}
              <input
                v-model.number="form.schedule_interval_min"
                type="number"
                min="60"
                max="10080"
                class="th-input mt-1 w-full"
              />
              <span class="mt-1 block text-slate-600">{{ t('adminPricingSync.scheduleIntervalHint') }}</span>
            </label>
            <label class="flex items-center gap-2 text-sm">
              <input v-model="form.enabled" type="checkbox" class="rounded" />
              {{ t('adminPricingSync.enabled') }}
            </label>
          </div>
        </div>
        <div class="mt-6 flex justify-end gap-2">
          <button type="button" class="th-btn-secondary px-4 py-2 text-sm" @click="drawerOpen = false">{{ t('common.cancel') }}</button>
          <button type="button" class="th-btn-primary px-4 py-2 text-sm" :disabled="formSaving" @click="saveTarget">
            {{ formSaving ? t('common.submitting') : t('adminPricingSync.saveTarget') }}
          </button>
        </div>
      </div>
    </div>

    <!-- 同步弹窗 -->
    <div v-if="syncModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" @click.self="syncModalOpen = false">
      <div class="max-h-[90vh] w-full max-w-5xl overflow-auto rounded-xl border border-slate-600 bg-slate-900 p-5 shadow-xl">
        <h3 class="text-lg font-semibold">{{ syncModalTitle }}</h3>
        <p v-if="preview" class="mt-2 text-sm text-slate-400">
          {{ t('adminPricingSync.summary', { added: preview.added, updated: preview.updated, skipped: preview.skipped }) }}
          · {{ t('adminPricingSync.selectedCount', { n: selectedRowCount }) }}
        </p>
        <p v-if="preview?.expires_at" class="mt-1 text-xs text-slate-500">
          {{ t('adminPricingSync.previewExpires', { time: preview.expires_at }) }}
        </p>
        <p v-if="previewStale" class="mt-2 text-xs text-amber-400">{{ t('adminPricingSync.previewStaleHint') }}</p>
        <div v-if="preview" class="mt-4 flex gap-2 text-xs">
          <button type="button" class="th-btn-secondary px-2 py-1" @click="selectAllRows(true)">{{ t('adminPricingSync.selectAll') }}</button>
          <button type="button" class="th-btn-secondary px-2 py-1" @click="selectAllRows(false)">{{ t('adminPricingSync.selectNone') }}</button>
          <button type="button" class="th-btn-primary px-2 py-1" @click="loadPreview">{{ t('adminPricingSync.refreshPreview') }}</button>
        </div>
        <table v-if="preview" class="mt-3 w-full text-left text-xs">
          <thead>
            <tr class="text-slate-500">
              <th class="py-1 w-8"></th>
              <th class="py-1">{{ t('adminPricingSync.scopes') }}</th>
              <th class="py-1">model</th>
              <th class="py-1">{{ t('adminPricingSync.colChange') }}</th>
              <th class="py-1">{{ t('adminPricingSync.colDetail') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(r, idx) in preview.rows.slice(0, 100)" :key="idx" class="border-t border-slate-800">
              <td class="py-1"><input type="checkbox" :checked="r.selected" @change="toggleRow(r)" /></td>
              <td class="py-1">{{ layerLabel(r.layer) }}</td>
              <td class="py-1 font-mono">{{ r.model_name }}</td>
              <td class="py-1 whitespace-nowrap">{{ changeLabel(r.change) }}</td>
              <td class="py-2 align-top">
                <div v-if="r.field_changes?.length" class="flex flex-wrap gap-1.5">
                  <span
                    v-for="(fc, fi) in r.field_changes"
                    :key="fi"
                    class="inline-flex max-w-full flex-wrap items-center gap-x-1 rounded border border-slate-700/70 bg-slate-800/50 px-1.5 py-0.5 leading-relaxed"
                  >
                    <span class="shrink-0 text-slate-500">{{ fc.label }}</span>
                    <template v-if="fc.old">
                      <span class="text-slate-500 line-through decoration-slate-600">{{ fc.old }}</span>
                      <span class="text-slate-600">→</span>
                    </template>
                    <span class="font-medium text-slate-200">{{ fc.new }}</span>
                  </span>
                </div>
                <span v-else-if="r.detail" class="text-slate-400">{{ r.detail }}</span>
                <span
                  v-else-if="r.change === 'updated' || r.change === 'added'"
                  class="text-amber-400/90"
                >
                  {{ t('adminPricingSync.detailMissingHint') }}
                </span>
                <span v-else class="text-slate-500">—</span>
              </td>
            </tr>
          </tbody>
        </table>
        <label class="mt-4 flex items-start gap-2 text-sm">
          <input v-model="confirmChecked" type="checkbox" class="mt-1" />
          {{ t('adminPricingSync.confirmCheckbox') }}
        </label>
        <div class="mt-4 flex justify-end gap-2">
          <button type="button" class="th-btn-secondary px-4 py-2 text-sm" @click="syncModalOpen = false">{{ t('common.cancel') }}</button>
          <button type="button" class="th-btn-primary px-4 py-2 text-sm" :disabled="!canConfirmSync" @click="confirmSync">
            {{ syncing ? t('common.submitting') : t('adminPricingSync.confirmExecute') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
