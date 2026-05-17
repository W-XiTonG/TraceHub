<script setup lang="ts">
import axios from 'axios'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import { quotaPer1KFromUpstreamPrice } from '../utils/upstreamQuotaDerive'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'

type SitePayload = {
  currency_quota_per_yuan?: number
  currency_quota_per_yuan_file?: number
  currency_quota_per_yuan_db_set?: boolean
  currency_quota_per_usd?: number
  currency_quota_per_usd_file?: number
  currency_quota_per_usd_db_set?: boolean
  pricing_cny_per_usd?: number
  pricing_cny_per_usd_file?: number
  pricing_cny_per_usd_db_set?: boolean
  default_pricing_markup_percent?: number
  default_pricing_markup_percent_file?: number
  default_pricing_markup_percent_db_set?: boolean
}

const { t } = useI18n()
const { busy, visible: busyUi, run: runLoad } = useSmoothLoading()
const saving = ref(false)
const resetting = ref(false)
const draftYuan = ref('')
const draftUsd = ref('')
const draftRate = ref('')
const draftMarkup = ref('')
const fileYuan = ref(0)
const fileUsd = ref(0)
const fileRate = ref(0)
const dbYuan = ref(false)
const dbUsd = ref(false)
const dbRate = ref(false)
const fileMarkup = ref(0)
const dbMarkup = ref(false)
const flashMsg = ref('')
const flashIsError = ref(false)
const FLASH_DISMISS_MS = 3200
const FLASH_ERROR_DISMISS_MS = 5500
let flashDismissTimer: ReturnType<typeof setTimeout> | undefined

function clearFlashDismissTimer() {
  if (flashDismissTimer !== undefined) {
    clearTimeout(flashDismissTimer)
    flashDismissTimer = undefined
  }
}

function clearFlash() {
  clearFlashDismissTimer()
  flashMsg.value = ''
  flashIsError.value = false
}

function showFlash(text: string, isError: boolean) {
  clearFlashDismissTimer()
  flashIsError.value = isError
  flashMsg.value = text
  const ms = isError ? FLASH_ERROR_DISMISS_MS : FLASH_DISMISS_MS
  const shown = text
  flashDismissTimer = setTimeout(() => {
    flashDismissTimer = undefined
    if (flashMsg.value === shown) clearFlash()
  }, ms)
}

function httpErr(e: unknown): string {
  if (axios.isAxiosError(e) && e.response?.data && typeof e.response.data === 'object') {
    const d = e.response.data as { message?: string }
    if (typeof d.message === 'string' && d.message.trim()) return d.message.trim()
  }
  return e instanceof Error ? e.message : t('adminCommon.requestFail')
}

function flashSuccess(text: string) {
  showFlash(text, false)
}

function flashError(e: unknown) {
  showFlash(httpErr(e), true)
}

function flashWarn(text: string) {
  showFlash(text, true)
}

function applyPayload(d: SitePayload) {
  const y = typeof d.currency_quota_per_yuan === 'number' ? d.currency_quota_per_yuan : 0
  const u = typeof d.currency_quota_per_usd === 'number' ? d.currency_quota_per_usd : 0
  const r = typeof d.pricing_cny_per_usd === 'number' ? d.pricing_cny_per_usd : 0
  draftYuan.value = String(y)
  draftUsd.value = String(u)
  draftRate.value = String(r)
  fileYuan.value = typeof d.currency_quota_per_yuan_file === 'number' ? d.currency_quota_per_yuan_file : 0
  fileUsd.value = typeof d.currency_quota_per_usd_file === 'number' ? d.currency_quota_per_usd_file : 0
  fileRate.value = typeof d.pricing_cny_per_usd_file === 'number' ? d.pricing_cny_per_usd_file : 0
  dbYuan.value = Boolean(d.currency_quota_per_yuan_db_set)
  dbUsd.value = Boolean(d.currency_quota_per_usd_db_set)
  dbRate.value = Boolean(d.pricing_cny_per_usd_db_set)
  const m =
    typeof d.default_pricing_markup_percent === 'number' ? d.default_pricing_markup_percent : 0
  draftMarkup.value = String(m)
  fileMarkup.value =
    typeof d.default_pricing_markup_percent_file === 'number'
      ? d.default_pricing_markup_percent_file
      : 0
  dbMarkup.value = Boolean(d.default_pricing_markup_percent_db_set)
}

async function load() {
  await runLoad(async () => {
    const { data } = await http.get<Envelope<SitePayload>>('/api/admin/site-settings')
    if (data.code !== 0 || !data.data) throw new Error(data.message || 'load')
    applyPayload(data.data)
  }).catch((e: unknown) => {
    flashError(e)
  })
}

const parsedYuan = computed(() => {
  const n = Number(draftYuan.value.trim())
  return Number.isFinite(n) && Number.isInteger(n) && n > 0 ? n : null
})

const parsedUsd = computed(() => {
  const n = Number(draftUsd.value.trim())
  return Number.isFinite(n) && Number.isInteger(n) && n >= 0 ? n : null
})

const parsedRate = computed(() => {
  const n = Number(draftRate.value.trim())
  return Number.isFinite(n) && n >= 0 ? n : null
})

const parsedMarkup = computed(() => {
  const n = Number(draftMarkup.value.trim())
  return Number.isFinite(n) && n > -100 ? n : null
})

const siteFactors = computed(() => ({
  quotaPerYuan: parsedYuan.value ?? 0,
  quotaPerUsd: parsedUsd.value ?? 0,
  cnyPerUsd: parsedRate.value ?? 0,
}))

const exampleCnyQuota = computed(() => {
  if (parsedYuan.value === null) return null
  return quotaPer1KFromUpstreamPrice(siteFactors.value, 'cny', 1_000_000, 10, 0)
})

const usdModeHintKey = computed(() => {
  if (parsedUsd.value !== null && parsedUsd.value > 0) return 'usdModeQuotaPerUsd'
  if (parsedRate.value !== null && parsedRate.value > 0 && parsedYuan.value !== null) return 'usdModeViaRate'
  return 'usdModeUnset'
})

function validateDraft(): string | null {
  if (parsedYuan.value === null) return t('adminPricingConversion.cfgErrYuan')
  if (parsedUsd.value === null) return t('adminPricingConversion.cfgErrUsd')
  if (parsedRate.value === null) return t('adminPricingConversion.cfgErrRate')
  if (parsedMarkup.value === null) return t('adminPricingConversion.cfgErrMarkup')
  return null
}

async function save() {
  clearFlash()
  const err = validateDraft()
  if (err) {
    flashWarn(err)
    return
  }
  saving.value = true
  try {
    const { data } = await http.put<Envelope<unknown>>('/api/admin/config', {
      items: [
        {
          key: 'site.currency_quota_per_yuan',
          value: String(parsedYuan.value),
          description: t('adminPricingConversion.cfgDescYuan'),
        },
        {
          key: 'site.currency_quota_per_usd',
          value: String(parsedUsd.value),
          description: t('adminPricingConversion.cfgDescUsd'),
        },
        {
          key: 'site.pricing_cny_per_usd',
          value: String(parsedRate.value),
          description: t('adminPricingConversion.cfgDescRate'),
        },
        {
          key: 'site.default_pricing_markup_percent',
          value: String(parsedMarkup.value),
          description: t('adminPricingConversion.cfgDescMarkup'),
        },
      ],
    })
    if (data.code !== 0) throw new Error(data.message || 'save')
    await load()
    flashSuccess(t('adminPricingConversion.cfgSaved'))
  } catch (e: unknown) {
    flashError(e)
  } finally {
    saving.value = false
  }
}

async function resetToYaml() {
  clearFlash()
  if (!confirm(t('adminPricingConversion.resetConfirm'))) return
  resetting.value = true
  try {
    const { data } = await http.post<Envelope<unknown>>('/api/admin/site-settings/reset-currency-pricing')
    if (data.code !== 0) throw new Error(data.message || 'reset')
    await load()
    flashSuccess(t('adminPricingConversion.resetDone'))
  } catch (e: unknown) {
    flashError(e)
  } finally {
    resetting.value = false
  }
}

onMounted(() => {
  void load()
})

onUnmounted(() => {
  clearFlashDismissTimer()
})
</script>

<template>
  <div>
    <article
      class="overflow-hidden rounded-2xl border border-slate-800/90 bg-slate-900/60 ring-1 ring-white/[0.04]"
    >
      <div class="border-b border-slate-800/80 px-5 py-4">
        <h3 class="text-lg font-semibold text-white">{{ t('adminPricingConversion.title') }}</h3>
        <p class="mt-2 text-sm leading-relaxed text-slate-400">{{ t('adminPricingConversion.intro') }}</p>
        <p class="mt-2 text-xs leading-relaxed text-slate-500">
          <RouterLink
            :to="{ name: 'admin-billing-global-pricing' }"
            class="text-cyan-400/90 underline-offset-2 hover:text-cyan-300 hover:underline"
          >
            {{ t('adminPricingConversion.backToPricing') }}
          </RouterLink>
        </p>
      </div>

      <div class="border-t border-slate-800/80 px-5 py-5">
        <ThLoadingState v-if="busyUi" />
        <template v-else>
          <p class="mb-4 text-xs leading-relaxed text-slate-500">{{ t('adminPricingConversion.formHint') }}</p>

          <div class="grid max-w-2xl gap-5 sm:grid-cols-1">
            <div>
              <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">
                {{ t('adminPricingConversion.cfgQuotaPerYuan') }}
              </label>
              <input
                v-model="draftYuan"
                type="text"
                inputmode="numeric"
                class="th-input w-full max-w-xs font-mono text-sm tabular-nums"
                autocomplete="off"
              />
              <p class="mt-1.5 text-[11px] text-slate-500">
                {{ t('adminPricingConversion.cfgYamlYuan', { y: fileYuan }) }}
              </p>
              <p v-if="dbYuan" class="mt-1 text-[11px] text-amber-100/80">
                {{ t('adminPricingConversion.cfgDbOverrideYuan') }}
              </p>
            </div>

            <div>
              <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">
                {{ t('adminPricingConversion.cfgQuotaPerUsd') }}
              </label>
              <input
                v-model="draftUsd"
                type="text"
                inputmode="numeric"
                class="th-input w-full max-w-xs font-mono text-sm tabular-nums"
                autocomplete="off"
              />
              <p class="mt-1.5 text-[11px] text-slate-500">{{ t('adminPricingConversion.cfgUsdHint') }}</p>
              <p class="mt-1 text-[11px] text-slate-500">
                {{ t('adminPricingConversion.cfgYamlUsd', { u: fileUsd }) }}
              </p>
              <p v-if="dbUsd" class="mt-1 text-[11px] text-amber-100/80">
                {{ t('adminPricingConversion.cfgDbOverrideUsd') }}
              </p>
            </div>

            <div>
              <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">
                {{ t('adminPricingConversion.cfgDefaultMarkup') }}
              </label>
              <input
                v-model="draftMarkup"
                type="text"
                inputmode="decimal"
                class="th-input w-full max-w-xs font-mono text-sm tabular-nums"
                autocomplete="off"
              />
              <p class="mt-1.5 text-[11px] text-slate-500">{{ t('adminPricingConversion.cfgMarkupHint') }}</p>
              <p class="mt-1 text-[11px] text-slate-500">
                {{ t('adminPricingConversion.cfgYamlMarkup', { m: fileMarkup }) }}
              </p>
              <p v-if="dbMarkup" class="mt-1 text-[11px] text-amber-100/80">
                {{ t('adminPricingConversion.cfgDbOverrideMarkup') }}
              </p>
            </div>

            <div>
              <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">
                {{ t('adminPricingConversion.cfgCnyPerUsd') }}
              </label>
              <input
                v-model="draftRate"
                type="text"
                inputmode="decimal"
                class="th-input w-full max-w-xs font-mono text-sm tabular-nums"
                autocomplete="off"
              />
              <p class="mt-1.5 text-[11px] text-slate-500">
                {{ t('adminPricingConversion.cfgYamlRate', { r: fileRate }) }}
              </p>
              <p v-if="dbRate" class="mt-1 text-[11px] text-amber-100/80">
                {{ t('adminPricingConversion.cfgDbOverrideRate') }}
              </p>
            </div>
          </div>

          <section class="th-glass mt-6 max-w-2xl rounded-xl p-4 text-xs leading-relaxed text-slate-400">
            <h4 class="text-sm font-medium text-slate-200">{{ t('adminPricingConversion.workflowTitle') }}</h4>
            <p class="mt-2 text-slate-300">{{ t('adminPricingConversion.workflowSteps') }}</p>
            <h4 class="mt-4 text-sm font-medium text-slate-200">{{ t('adminPricingConversion.guideTitle') }}</h4>
            <p class="mt-2">{{ t('adminPricingConversion.conversionIntro') }}</p>
            <p class="mt-2 font-mono text-[11px] text-slate-500">{{ t('adminPricingConversion.conversionCnyFormula') }}</p>
            <p v-if="exampleCnyQuota !== null" class="mt-2 text-slate-300">
              {{
                t('adminPricingConversion.conversionExample', {
                  price: 10,
                  unit: 'CNY/1M',
                  quota: exampleCnyQuota,
                  y: parsedYuan,
                })
              }}
            </p>
            <p class="mt-2">
              {{
                t(`adminPricingConversion.${usdModeHintKey}`, {
                  u: parsedUsd ?? 0,
                  r: parsedRate ?? 0,
                  y: parsedYuan ?? 0,
                })
              }}
            </p>
          </section>

          <div class="mt-6 flex flex-wrap gap-3">
            <button
              type="button"
              class="th-pg-btn-primary px-5 py-2.5 text-sm"
              :disabled="saving || resetting"
              @click="save"
            >
              {{ saving ? t('common.submitting') : t('adminPricingConversion.cfgSave') }}
            </button>
            <button
              type="button"
              class="th-btn-secondary px-5 py-2.5 text-sm"
              :disabled="saving || resetting || busy"
              @click="load"
            >
              {{ t('adminCommon.refresh') }}
            </button>
            <button
              type="button"
              class="th-btn-secondary border-amber-500/25 px-5 py-2.5 text-sm text-amber-100/90 hover:border-amber-500/40"
              :disabled="saving || resetting"
              @click="resetToYaml"
            >
              {{ resetting ? t('common.loading') : t('adminPricingConversion.resetDefaults') }}
            </button>
          </div>
        </template>
      </div>
    </article>

    <Teleport to="body">
      <Transition name="th-admin-flash-fade">
        <div
          v-if="flashMsg"
          class="pointer-events-none fixed left-1/2 top-20 z-[10050] max-w-[min(22rem,calc(100vw-2rem))] -translate-x-1/2 rounded-lg border px-4 py-2.5 text-center text-xs shadow-lg backdrop-blur-sm"
          :class="flashIsError ? 'th-admin-flash--error' : 'th-admin-flash--success'"
          role="status"
          aria-live="polite"
        >
          {{ flashMsg }}
        </div>
      </Transition>
    </Teleport>
  </div>
</template>
