<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import {
  formatModelPricingDisplay,
  modelHasDisplayAlias,
  modelTitle,
  type ModelPricing,
  type PriceCurrency,
  type SiteCurrencyFactors,
} from '../utils/modelPricingDisplay'
import MobileLabelValueCard from '../components/ui/MobileLabelValueCard.vue'
import MobileLabelValueRow from '../components/ui/MobileLabelValueRow.vue'

const { t } = useI18n()

type ModelItem = {
  id: string
  object?: string
  created?: number
  owned_by?: string
  display_name?: string
  in_gateway?: boolean
  in_catalog?: boolean
  upstream_list_status?: string
  upstream_list_checked_at?: string
  labels?: string[]
  pricing?: ModelPricing
}

type ModelsPayload = {
  items: ModelItem[]
  total: number
  group_id: number
  group_assigned: boolean
  hint?: string
  currency_quota_per_yuan?: number
  currency_quota_per_usd?: number
  pricing_cny_per_usd?: number
}

const PRICE_CURRENCY_KEY = 'tracehub.models.priceCurrency'

const payload = ref<ModelsPayload | null>(null)
const err = ref('')
const q = ref('')
const priceCurrency = ref<PriceCurrency>(
  (typeof localStorage !== 'undefined' && localStorage.getItem(PRICE_CURRENCY_KEY) === 'cny'
    ? 'cny'
    : 'usd') as PriceCurrency,
)

const currencyFactors = computed((): SiteCurrencyFactors => {
  const p = payload.value
  const quotaPerYuan =
    typeof p?.currency_quota_per_yuan === 'number' && p.currency_quota_per_yuan > 0
      ? p.currency_quota_per_yuan
      : 10000
  const quotaPerUsd =
    typeof p?.currency_quota_per_usd === 'number' && p.currency_quota_per_usd > 0
      ? p.currency_quota_per_usd
      : 0
  const cnyPerUsd =
    typeof p?.pricing_cny_per_usd === 'number' && p.pricing_cny_per_usd > 0 ? p.pricing_cny_per_usd : 0
  return { quotaPerYuan, quotaPerUsd, cnyPerUsd }
})

const pricingLabels = computed(() => ({
  free: t('modelsPage.priceFree'),
  in: t('modelsPage.priceIn'),
  out: t('modelsPage.priceOut'),
  unified: t('modelsPage.priceUnified'),
  perMillion: t('modelsPage.pricePerMillion'),
  quotaPer1k: t('modelsPage.priceQuotaPer1k'),
  quotaUnit: t('modelsPage.priceQuotaUnit'),
  symbolUsd: '$',
  symbolCny: '¥',
}))

function setPriceCurrency(c: PriceCurrency) {
  priceCurrency.value = c
  try {
    localStorage.setItem(PRICE_CURRENCY_KEY, c)
  } catch {
    /* ignore */
  }
}

function modelPricingCell(m: ModelItem) {
  return formatModelPricingDisplay(m.pricing, priceCurrency.value, currencyFactors.value, pricingLabels.value)
}

const filtered = computed(() => {
  const items = payload.value?.items ?? []
  const s = q.value.trim().toLowerCase()
  if (!s) return items
  return items.filter((m) => {
    if (
      m.id.toLowerCase().includes(s) ||
      (m.owned_by ?? '').toLowerCase().includes(s) ||
      (m.display_name ?? '').toLowerCase().includes(s) ||
      modelTitle(m.display_name, m.id).toLowerCase().includes(s) ||
      (m.upstream_list_status ?? '').toLowerCase().includes(s)
    ) {
      return true
    }
    if (modelPricingCell(m).searchBlob.toLowerCase().includes(s)) return true
    if (upstreamListCell(m).text.toLowerCase().includes(s)) return true
    const lb = modelLabelsSearchBlob(m)
    return lb.includes(s)
  })
})

function modelKindDisplayLabel(key: string): string {
  switch (key) {
    case 'chat':
      return t('modelsPage.labelChat')
    case 'embedding':
      return t('modelsPage.labelEmbedding')
    case 'image':
      return t('modelsPage.labelImage')
    case 'video':
      return t('modelsPage.labelVideo')
    case 'audio':
      return t('modelsPage.labelAudio')
    case 'moderation':
      return t('modelsPage.labelModeration')
    case 'rerank':
      return t('modelsPage.labelRerank')
    default:
      return key
  }
}

function modelLabelsSearchBlob(m: ModelItem): string {
  return (m.labels ?? []).map((k) => modelKindDisplayLabel(k).toLowerCase()).join(' ')
}

function upstreamListCell(m: ModelItem): {
  text: string
  title: string
  variant: 'unknown' | 'available' | 'unavailable' | 'partial'
  pillClass: string
  dotClass: string
} {
  const unknownPill =
    'th-models-upstream-pill th-models-upstream-pill--unknown border border-dashed border-slate-600/55 bg-slate-950/70 text-slate-500 shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]'
  if (!m.in_gateway) {
    return {
      text: t('modelsPage.upstreamUnknown'),
      title: '',
      variant: 'unknown',
      pillClass: unknownPill,
      dotClass: 'th-models-upstream-pill-dot bg-slate-600',
    }
  }
  const st = (m.upstream_list_status ?? '').trim()
  const checked = (m.upstream_list_checked_at ?? '').trim()
  const hint = t('modelsPage.thUpstreamListHint')
  const timeTitle = checked ? `${checked}\n${hint}` : hint
  switch (st) {
    case 'listed':
      return {
        text: t('modelsPage.upstreamAvailable'),
        title: timeTitle,
        variant: 'available',
        pillClass:
          'th-models-upstream-pill th-models-upstream-pill--available border border-emerald-400/25 bg-gradient-to-br from-emerald-500/[0.22] via-teal-500/10 to-slate-900/40 text-emerald-50 ring-1 ring-inset ring-white/[0.06] shadow-[0_0_28px_-12px_rgba(16,185,129,0.55),inset_0_1px_0_rgba(255,255,255,0.06)]',
        dotClass: 'th-models-upstream-pill-dot bg-emerald-400 shadow-[0_0_10px_rgba(52,211,153,0.75)]',
      }
    case 'not_listed':
      return {
        text: t('modelsPage.upstreamUnavailable'),
        title: timeTitle,
        variant: 'unavailable',
        pillClass:
          'th-models-upstream-pill th-models-upstream-pill--unavailable border border-rose-400/25 bg-gradient-to-br from-rose-500/[0.18] via-fuchsia-900/10 to-slate-900/45 text-rose-50 ring-1 ring-inset ring-white/[0.05] shadow-[0_0_26px_-12px_rgba(244,63,94,0.45),inset_0_1px_0_rgba(255,255,255,0.05)]',
        dotClass: 'th-models-upstream-pill-dot bg-rose-400 shadow-[0_0_10px_rgba(251,113,133,0.55)]',
      }
    case 'mixed':
      return {
        text: t('modelsPage.upstreamPartial'),
        title: timeTitle,
        variant: 'partial',
        pillClass:
          'th-models-upstream-pill th-models-upstream-pill--partial border border-amber-400/30 bg-gradient-to-br from-amber-500/[0.2] via-violet-600/10 to-slate-900/40 text-amber-50 ring-1 ring-inset ring-white/[0.06] shadow-[0_0_28px_-12px_rgba(245,158,11,0.4),inset_0_1px_0_rgba(255,255,255,0.06)]',
        dotClass: 'th-models-upstream-pill-dot bg-gradient-to-br from-amber-300 to-violet-400 shadow-[0_0_10px_rgba(251,191,36,0.5)]',
      }
    default:
      return {
        text: t('modelsPage.upstreamUnknown'),
        title: hint,
        variant: 'unknown',
        pillClass: unknownPill,
        dotClass: 'th-models-upstream-pill-dot bg-slate-500',
      }
  }
}

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
        <div
          class="inline-flex rounded-lg border border-white/[0.08] bg-slate-950/60 p-0.5"
          role="group"
          :aria-label="t('modelsPage.priceCurrencyAria')"
        >
          <button
            type="button"
            class="rounded-md px-2.5 py-1 text-xs font-medium transition"
            :class="
              priceCurrency === 'usd'
                ? 'bg-cyan-500/20 text-cyan-100 shadow-sm ring-1 ring-cyan-400/30'
                : 'text-slate-500 hover:text-slate-300'
            "
            @click="setPriceCurrency('usd')"
          >
            {{ t('modelsPage.priceCurrencyUsd') }}
          </button>
          <button
            type="button"
            class="rounded-md px-2.5 py-1 text-xs font-medium transition"
            :class="
              priceCurrency === 'cny'
                ? 'bg-cyan-500/20 text-cyan-100 shadow-sm ring-1 ring-cyan-400/30'
                : 'text-slate-500 hover:text-slate-300'
            "
            @click="setPriceCurrency('cny')"
          >
            {{ t('modelsPage.priceCurrencyCny') }}
          </button>
        </div>
        <span class="text-xs text-slate-500">{{ t('modelsPage.count', { total: payload.total, n: filtered.length }) }}</span>
      </div>

      <div class="th-glass mt-6 flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        <div class="hidden min-h-0 flex-1 flex-col overflow-hidden lg:flex">
        <div class="th-table-wrap th-table-wrap--scrollable thin-scrollbar min-h-0 flex-1">
          <table class="w-full text-left text-sm">
            <thead class="sticky top-0 z-10">
              <tr class="border-b border-white/[0.06] bg-slate-900/95 text-xs uppercase tracking-wider text-slate-500 shadow-[0_1px_0_rgba(0,0,0,0.35)] backdrop-blur-sm">
                <th class="px-4 py-3 font-medium">{{ t('modelsPage.thModel') }}</th>
                <th class="px-4 py-3 font-medium" :title="t('modelsPage.priceHint')">{{ t('modelsPage.thPrice') }}</th>
                <th class="px-4 py-3 font-medium" :title="t('modelsPage.labelsHint')">{{ t('modelsPage.thLabels') }}</th>
                <th class="px-4 py-3 font-medium">{{ t('modelsPage.thSource') }}</th>
                <th class="px-4 py-3 font-medium" :title="t('modelsPage.thUpstreamListHint')">{{ t('modelsPage.thUpstreamList') }}</th>
                <th class="px-4 py-3 font-medium">{{ t('modelsPage.thOwner') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-white/[0.06]">
              <tr v-for="m in filtered" :key="m.id" class="transition hover:bg-white/[0.03]">
                <td class="px-4 py-3">
                  <div class="font-medium text-slate-200">{{ modelTitle(m.display_name, m.id) }}</div>
                  <div
                    v-if="modelHasDisplayAlias(m.display_name)"
                    class="mt-0.5 font-mono text-xs text-slate-500"
                    :title="m.id"
                  >
                    {{ m.id }}
                  </div>
                </td>
                <td class="px-4 py-3" :title="t('modelsPage.priceHint')">
                  <template v-for="price in [modelPricingCell(m)]" :key="`${m.id}-price`">
                    <span
                      v-if="price.free"
                      class="inline-flex rounded-full border border-emerald-500/25 bg-emerald-950/40 px-2 py-0.5 text-xs font-medium text-emerald-300/95"
                    >
                      {{ price.fiatPrimary }}
                    </span>
                    <template v-else-if="price.fiatPrimary">
                      <p class="text-sm tabular-nums text-amber-100/90">{{ price.fiatPrimary }}</p>
                      <p v-if="price.quotaSecondary" class="mt-0.5 text-xs tabular-nums text-slate-500">
                        {{ price.quotaSecondary }}
                      </p>
                    </template>
                    <span v-else class="text-xs text-slate-600">{{ t('modelsPage.dash') }}</span>
                  </template>
                </td>
                <td class="px-4 py-3">
                  <div class="flex flex-wrap gap-1">
                    <span
                      v-for="lab in m.labels ?? []"
                      :key="`${m.id}-kind-${lab}`"
                      class="th-model-kind-pill"
                      :title="t('modelsPage.labelsHint')"
                    >
                      {{ modelKindDisplayLabel(lab) }}
                    </span>
                    <span v-if="!(m.labels && m.labels.length)" class="text-xs text-slate-600">{{ t('modelsPage.dash') }}</span>
                  </div>
                </td>
                <td class="px-4 py-3">
                  <span v-if="m.in_gateway && m.in_catalog" class="text-xs text-emerald-400/90">{{ t('modelsPage.srcBoth') }}</span>
                  <span v-else-if="m.in_gateway" class="text-xs text-cyan-400/90">{{ t('modelsPage.srcGw') }}</span>
                  <span v-else class="text-xs text-slate-500">{{ t('modelsPage.srcCat') }}</span>
                </td>
                <td class="px-4 py-3">
                  <span
                    v-for="cell in [upstreamListCell(m)]"
                    :key="`${m.id}-upstream`"
                    class="inline-flex max-w-full items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-semibold leading-none tracking-wide tabular-nums backdrop-blur-[2px] transition-[box-shadow,transform] duration-200 hover:brightness-110"
                    :class="cell.pillClass"
                    :data-upstream-variant="cell.variant"
                    :title="cell.title"
                  >
                    <span
                      class="h-1.5 w-1.5 shrink-0 rounded-full ring-1 ring-black/20"
                      :class="cell.dotClass"
                      aria-hidden="true"
                    />
                    <span class="min-w-0 truncate">{{ cell.text }}</span>
                  </span>
                </td>
                <td class="px-4 py-3 text-slate-400">{{ m.owned_by ?? t('modelsPage.dash') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        </div>

        <div class="thin-scrollbar space-y-3 overflow-y-auto p-3 lg:hidden">
          <MobileLabelValueCard v-for="m in filtered" :key="`card-${m.id}`">
            <template #title>
              <p class="font-medium text-slate-200">{{ modelTitle(m.display_name, m.id) }}</p>
              <p v-if="modelHasDisplayAlias(m.display_name)" class="mt-0.5 font-mono text-xs text-slate-500">{{ m.id }}</p>
            </template>
            <MobileLabelValueRow :label="t('modelsPage.thPrice')">
              <template v-for="price in [modelPricingCell(m)]" :key="`${m.id}-mprice`">
                <span
                  v-if="price.free"
                  class="inline-flex rounded-full border border-emerald-500/25 bg-emerald-950/40 px-2 py-0.5 text-xs font-medium text-emerald-300/95"
                >
                  {{ price.fiatPrimary }}
                </span>
                <template v-else-if="price.fiatPrimary">
                  <p class="text-sm tabular-nums text-amber-100/90">{{ price.fiatPrimary }}</p>
                  <p v-if="price.quotaSecondary" class="mt-0.5 text-xs tabular-nums text-slate-500">{{ price.quotaSecondary }}</p>
                </template>
                <span v-else class="text-xs text-slate-600">{{ t('modelsPage.dash') }}</span>
              </template>
            </MobileLabelValueRow>
            <MobileLabelValueRow :label="t('modelsPage.thLabels')">
              <div class="flex flex-wrap gap-1">
                <span v-for="lab in m.labels ?? []" :key="`${m.id}-mkind-${lab}`" class="th-model-kind-pill">{{ modelKindDisplayLabel(lab) }}</span>
                <span v-if="!(m.labels && m.labels.length)" class="text-xs text-slate-600">{{ t('modelsPage.dash') }}</span>
              </div>
            </MobileLabelValueRow>
            <MobileLabelValueRow :label="t('modelsPage.thSource')">
              <span v-if="m.in_gateway && m.in_catalog" class="text-xs text-emerald-400/90">{{ t('modelsPage.srcBoth') }}</span>
              <span v-else-if="m.in_gateway" class="text-xs text-cyan-400/90">{{ t('modelsPage.srcGw') }}</span>
              <span v-else class="text-xs text-slate-500">{{ t('modelsPage.srcCat') }}</span>
            </MobileLabelValueRow>
            <MobileLabelValueRow :label="t('modelsPage.thUpstreamList')">
              <span
                v-for="cell in [upstreamListCell(m)]"
                :key="`${m.id}-mupstream`"
                class="inline-flex max-w-full items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-semibold leading-none tracking-wide tabular-nums backdrop-blur-[2px]"
                :class="cell.pillClass"
                :title="cell.title"
              >
                <span class="h-1.5 w-1.5 shrink-0 rounded-full ring-1 ring-black/20" :class="cell.dotClass" aria-hidden="true" />
                <span class="min-w-0 truncate">{{ cell.text }}</span>
              </span>
            </MobileLabelValueRow>
            <MobileLabelValueRow :label="t('modelsPage.thOwner')">
              {{ m.owned_by ?? t('modelsPage.dash') }}
            </MobileLabelValueRow>
          </MobileLabelValueCard>
        </div>

        <p v-if="payload.total === 0" class="px-4 py-12 text-center text-sm text-slate-500">
          {{ t('modelsPage.emptyData') }}
        </p>
        <p v-else-if="filtered.length === 0" class="px-4 py-8 text-center text-sm text-slate-500">{{ t('modelsPage.emptyFilter') }}</p>
      </div>
    </template>
  </div>
</template>
