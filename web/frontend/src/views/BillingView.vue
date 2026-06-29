<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import { apiErrorMessage } from '../stores/auth'
import contactCardImg from '../assets/xunji-contact-card.png'
import { useSiteMetaStore, type RechargeShopItem } from '../stores/siteMeta'
import { checkoutQrImageSrc, paymentCheckoutUrl } from '../utils/paymentCheckout'

const { t } = useI18n()
const route = useRoute()
const siteMeta = useSiteMetaStore()

const shopMode = computed(() => siteMeta.rechargeViaExternalShop && siteMeta.rechargeShops != null)
const onlinePay = computed(() => !shopMode.value && siteMeta.onlinePaymentEnabled)
const shopTitle = computed(() => {
  const t0 = siteMeta.rechargeShops?.title?.trim()
  return t0 || t('billingPage.shopDefaultTitle')
})
const shopHint = computed(() => {
  const h = siteMeta.rechargeShops?.hint?.trim()
  return h || t('billingPage.shopDefaultHint')
})
const shopItems = computed(() => siteMeta.rechargeShops?.shops ?? [])

function openShop(url: string) {
  if (url) window.open(url, '_blank', 'noopener,noreferrer')
}

function shopQrImageSrc(shop: RechargeShopItem): string {
  if (!shop.show_qr) return ''
  const custom = shop.qr_image_url?.trim()
  if (custom) return custom
  return checkoutQrImageSrc(shop.url)
}

const shopQrFailed = ref<Record<number, boolean>>({})

function onShopQrError(idx: number) {
  shopQrFailed[idx] = true
}

type Package = {
  id: number
  name: string
  amount_cents: number
  quota_amount: number
  bonus_quota?: number
  currency: string
}

type PaymentProvider = {
  id: string
  label: string
  pay_types?: string[]
}

type OrderResult = {
  order_no: string
  status: string
  pay_url?: string
  qr_code?: string
  amount_cents: number
  quota_amount: number
  expire_at: string
}

const packages = ref<Package[]>([])
const providers = ref<PaymentProvider[]>([])
const quotaPerYuan = ref(10000)
const selectedPkg = ref<number | null>(null)
const provider = ref('')
const easypayType = ref('alipay')
const customYuan = ref('')
const useCustom = ref(false)
const loading = ref(false)
const msg = ref('')
const msgTone = ref<'success' | 'error'>('success')
const activeOrder = ref<OrderResult | null>(null)
let pollTimer: ReturnType<typeof setInterval> | null = null

const showEasypayType = computed(() => provider.value === 'easypay')

const payCheckoutUrl = computed(() => paymentCheckoutUrl(activeOrder.value))

const payQrImageSrc = computed(() => checkoutQrImageSrc(payCheckoutUrl.value))

const qrImgFailed = ref(false)

function openCheckout() {
  const url = payCheckoutUrl.value
  if (url) window.open(url, '_blank', 'noopener,noreferrer')
}

const easypayTypeOptions = computed(() => {
  const p = providers.value.find((x) => x.id === 'easypay')
  const types = p?.pay_types?.length ? p.pay_types : ['alipay', 'wxpay']
  return types.map((id) => ({
    id,
    label: id === 'wxpay' ? t('billingPage.wxpay') : t('billingPage.alipay'),
  }))
})

function fmtYuan(cents: number) {
  return (cents / 100).toFixed(2)
}

const quotaExample100 = computed(() => Math.round((100 * 100 * quotaPerYuan.value) / 100))

const customQuotaEstimate = computed(() => {
  if (!useCustom.value) return null
  const yuan = parseFloat(customYuan.value)
  if (!Number.isFinite(yuan) || yuan <= 0) return null
  const cents = Math.round(yuan * 100)
  return Math.round((cents * quotaPerYuan.value) / 100)
})

async function loadQuotaPerYuan() {
  try {
    const { data } = await http.get<Envelope<{ currency_quota_per_yuan?: number }>>('/api/user/me')
    if (data.code === 0 && typeof data.data?.currency_quota_per_yuan === 'number' && data.data.currency_quota_per_yuan > 0) {
      quotaPerYuan.value = data.data.currency_quota_per_yuan
    }
  } catch {
    /* ignore */
  }
}

async function loadProviders() {
  try {
    const { data } = await http.get<Envelope<{ providers: PaymentProvider[] }>>('/api/user/payment/providers')
    if (data.code === 0 && data.data?.providers) {
      providers.value = data.data.providers
      if (providers.value.length && !providers.value.some((p) => p.id === provider.value)) {
        provider.value = providers.value[0].id
      }
      const ep = providers.value.find((p) => p.id === 'easypay')
      if (ep?.pay_types?.length && !ep.pay_types.includes(easypayType.value)) {
        easypayType.value = ep.pay_types[0]
      }
    }
  } catch {
    providers.value = []
  }
}

async function loadPackages() {
  if (!onlinePay.value) return
  try {
    const { data } = await http.get<Envelope<{ items: Package[] }>>('/api/user/payment/packages')
    if (data.code === 0 && data.data?.items) {
      packages.value = data.data.items
      if (packages.value.length && selectedPkg.value == null) {
        selectedPkg.value = packages.value[0].id
      }
    }
  } catch {
    /* ignore */
  }
}

async function createOrder() {
  msg.value = ''
  loading.value = true
  activeOrder.value = null
  try {
    if (!provider.value) {
      msgTone.value = 'error'
      msg.value = t('billingPage.pickProvider')
      return
    }
    const body: Record<string, unknown> = { provider: provider.value }
    if (provider.value === 'easypay') {
      body.easypay_type = easypayType.value
    }
    if (useCustom.value) {
      const yuan = parseFloat(customYuan.value)
      if (!Number.isFinite(yuan) || yuan <= 0) {
        msgTone.value = 'error'
        msg.value = t('billingPage.invalidAmount')
        return
      }
      body.custom_amount_cents = Math.round(yuan * 100)
      body.currency = 'CNY'
    } else if (selectedPkg.value) {
      body.package_id = selectedPkg.value
    } else {
      msgTone.value = 'error'
      msg.value = t('billingPage.pickPackage')
      return
    }
    const { data } = await http.post<Envelope<OrderResult>>('/api/user/payment/orders', body)
    if (data.code !== 0 || !data.data) {
      msgTone.value = 'error'
      msg.value = apiErrorMessage(data) || t('billingPage.orderFail')
      return
    }
    const order = data.data
    if (!order.pay_url && !order.qr_code) {
      msgTone.value = 'error'
      msg.value = t('billingPage.noPayLink')
      return
    }
    activeOrder.value = order
    qrImgFailed.value = false
    msgTone.value = 'success'
    const checkout = paymentCheckoutUrl(order)
    if (checkout) {
      msg.value = t('billingPage.orderCreatedOpenPay')
      window.open(checkout, '_blank', 'noopener,noreferrer')
    } else {
      msg.value = t('billingPage.orderCreated')
    }
    startPoll(order.order_no)
  } catch (e: unknown) {
    msgTone.value = 'error'
    msg.value = apiErrorMessage(e) || t('billingPage.orderFail')
  } finally {
    loading.value = false
  }
}

function startPoll(orderNo: string) {
  stopPoll()
  pollTimer = setInterval(async () => {
    try {
      const { data } = await http.get<Envelope<{ status: string }>>(`/api/user/payment/orders/${orderNo}`)
      if (data.code === 0 && data.data?.status === 'paid') {
        msgTone.value = 'success'
        msg.value = t('billingPage.paidSuccess')
        stopPoll()
      }
    } catch {
      /* ignore */
    }
  }, 3000)
}

function stopPoll() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

async function mockPay() {
  if (!activeOrder.value) return
  try {
    await http.get('/webhook/payment/mock/pay', { params: { order_no: activeOrder.value.order_no } })
    msgTone.value = 'success'
    msg.value = t('billingPage.mockPaid')
  } catch {
    msgTone.value = 'error'
    msg.value = t('billingPage.orderFail')
  }
}

watch(provider, (id) => {
  if (id === 'easypay') {
    const ep = providers.value.find((p) => p.id === 'easypay')
    if (ep?.pay_types?.length && !ep.pay_types.includes(easypayType.value)) {
      easypayType.value = ep.pay_types[0]
    }
  }
})

onMounted(async () => {
  await siteMeta.ensureLoaded()
  void loadQuotaPerYuan()
  if (onlinePay.value) {
    await loadProviders()
    void loadPackages()
    if (route.query.paid === '1') {
      msgTone.value = 'success'
      msg.value = t('billingPage.paidSuccess')
    }
  }
})

onUnmounted(() => stopPoll())
</script>

<template>
  <div class="max-w-2xl min-w-0">
    <h1 class="mb-2 text-2xl font-bold">{{ t('billingPage.title') }}</h1>
    <p class="th-billing-panel__hint mb-6">
      {{
        shopMode
          ? shopHint
          : onlinePay
            ? t('billingPage.descOnline')
            : t('billingPage.desc')
      }}
    </p>

    <!-- 外部商铺充值模式 -->
    <template v-if="shopMode">
      <section class="th-console-panel mb-8 p-6">
        <h2 class="th-billing-panel__title">{{ shopTitle }}</h2>
        <div class="mt-5 grid gap-4">
          <article
            v-for="(shop, idx) in shopItems"
            :key="`${shop.url}-${idx}`"
            class="rounded-xl border border-cyan-500/25 bg-cyan-950/20 p-5"
          >
            <h3 class="text-base font-semibold text-cyan-100">{{ shop.label }}</h3>
            <p v-if="shop.description" class="mt-2 text-sm text-slate-400">{{ shop.description }}</p>
            <div
              v-if="shop.show_qr && shopQrImageSrc(shop) && !shopQrFailed[idx]"
              class="th-billing-qr-box mt-4 max-w-xs"
            >
              <p class="mb-3 text-center text-xs th-billing-panel__hint">{{ t('billingPage.shopScanQr') }}</p>
              <img
                :src="shopQrImageSrc(shop)"
                alt=""
                class="th-billing-qr-img mx-auto block"
                @error="onShopQrError(idx)"
              />
              <p class="mt-2 text-center text-xs text-slate-500">{{ t('billingPage.shopScanHint') }}</p>
            </div>
            <button type="button" class="th-btn-primary mt-4 text-sm" @click="openShop(shop.url)">
              {{ t('billingPage.shopBuyBtn') }}
            </button>
          </article>
        </div>
      </section>

      <div class="th-console-panel space-y-4 p-6">
        <div class="flex items-start gap-3">
          <span class="mt-0.5 text-cyan-400">①</span>
          <div>
            <p class="font-medium text-slate-200">{{ t('billingPage.shopStep1Title') }}</p>
            <p class="mt-1 text-sm text-slate-400">{{ t('billingPage.shopStep1Desc') }}</p>
          </div>
        </div>
        <div class="flex items-start gap-3">
          <span class="mt-0.5 text-cyan-400">②</span>
          <div>
            <p class="font-medium text-slate-200">{{ t('billingPage.shopStep2Title') }}</p>
            <p class="mt-1 text-sm text-slate-400">{{ t('billingPage.shopStep2Desc') }}</p>
          </div>
        </div>
        <div class="flex items-start gap-3">
          <span class="mt-0.5 text-cyan-400">③</span>
          <div>
            <p class="font-medium text-slate-200">{{ t('billingPage.redeemTitle') }}</p>
            <p class="mt-1 text-sm text-slate-400">{{ t('billingPage.shopStep3Desc') }}</p>
            <RouterLink to="/console/redeem" class="mt-2 inline-block text-sm text-cyan-400 hover:underline">
              {{ t('billingPage.redeemLink') }}
            </RouterLink>
          </div>
        </div>
      </div>
    </template>

    <!-- 在线支付关闭：原兑换码 + 企微 + 对公说明 -->
    <template v-else-if="!onlinePay">
      <section
        class="th-billing-wecom mb-8 rounded-2xl border-2 border-cyan-500/40 bg-gradient-to-br from-cyan-950/45 via-slate-900/80 to-slate-950/90 p-6 shadow-[0_20px_50px_-28px_rgba(34,211,238,0.35)] ring-1 ring-cyan-400/15"
        aria-labelledby="billing-wecom-heading"
      >
        <div class="flex flex-col gap-1 border-b border-cyan-500/20 pb-4 sm:flex-row sm:items-start sm:justify-between sm:gap-4">
          <div>
            <h2 id="billing-wecom-heading" class="text-base font-semibold tracking-tight text-cyan-100">
              {{ t('billingPage.wecomTitle') }}
            </h2>
            <p class="mt-2 max-w-xl text-sm leading-relaxed text-slate-300">{{ t('billingPage.wecomDesc') }}</p>
          </div>
          <span
            class="shrink-0 self-start rounded-full border border-amber-400/35 bg-amber-950/50 px-2.5 py-1 text-[10px] font-semibold uppercase tracking-wider text-amber-100/95"
          >
            {{ t('billingPage.wecomBadge') }}
          </span>
        </div>
        <div class="mt-5 flex flex-col items-center gap-3 sm:items-start">
          <div class="rounded-xl border-2 border-white/10 bg-white/[0.04] p-3 shadow-inner ring-2 ring-cyan-500/25">
            <img
              :src="siteMeta.displayContactCardUrl ?? contactCardImg"
              :alt="t('home.contactCardAlt')"
              width="300"
              loading="eager"
              decoding="async"
              class="mx-auto block h-auto w-full max-w-[min(100%,300px)] rounded-lg"
            />
          </div>
          <p class="max-w-md text-center text-xs leading-relaxed text-slate-400 sm:text-left">
            {{ t('home.contactScanHint') }}
          </p>
        </div>
      </section>

      <div class="th-console-panel space-y-4 p-6">
        <div class="flex items-start gap-3">
          <span class="mt-0.5 text-cyan-400">①</span>
          <div>
            <p class="font-medium text-slate-200">{{ t('billingPage.redeemTitle') }}</p>
            <p class="mt-1 text-sm text-slate-400">{{ t('billingPage.redeemDesc') }}</p>
            <RouterLink to="/console/redeem" class="mt-2 inline-block text-sm text-cyan-400 hover:underline">{{ t('billingPage.redeemLink') }}</RouterLink>
          </div>
        </div>
        <div class="flex items-start gap-3">
          <span class="mt-0.5 text-cyan-400">②</span>
          <div>
            <p class="font-medium text-slate-200">{{ t('billingPage.corpTitle') }}</p>
            <p class="mt-1 text-sm text-slate-400">{{ t('billingPage.corpDesc') }}</p>
          </div>
        </div>
        <div class="flex items-start gap-3 border-t border-slate-800 pt-4">
          <span class="mt-0.5 text-amber-500">!</span>
          <p class="text-sm text-slate-500">{{ t('billingPage.note') }}</p>
        </div>
      </div>
    </template>

    <!-- 在线支付开启 -->
    <template v-else>
      <section class="th-console-panel mb-8 p-6">
        <h2 class="th-billing-panel__title">{{ t('billingPage.packagesTitle') }}</h2>
        <p class="th-billing-panel__hint">
          {{ t('billingPage.quotaPerYuanHint', { perYuan: quotaPerYuan, example: quotaExample100 }) }}
        </p>
        <div v-if="packages.length" class="grid gap-3 sm:grid-cols-2">
          <label
            v-for="p in packages"
            :key="p.id"
            class="th-billing-package"
            :class="{ 'th-billing-package--selected': !useCustom && selectedPkg === p.id }"
          >
            <input v-model="selectedPkg" type="radio" class="sr-only" :value="p.id" @change="useCustom = false" />
            <p class="th-billing-package__name">{{ p.name }}</p>
            <p class="th-billing-package__price">¥{{ fmtYuan(p.amount_cents) }}</p>
            <p class="th-billing-package__quota">{{ t('billingPage.quotaLabel', { n: p.quota_amount + (p.bonus_quota || 0) }) }}</p>
          </label>
        </div>
        <p v-else class="th-billing-panel__hint text-sm">{{ t('billingPage.noPackages') }}</p>

        <div class="th-billing-divider flex flex-wrap items-center gap-3">
          <label class="flex items-center gap-2 text-sm th-billing-package__name">
            <input v-model="useCustom" type="checkbox" class="rounded" />
            {{ t('billingPage.customAmount') }}
          </label>
          <input
            v-if="useCustom"
            v-model="customYuan"
            type="number"
            min="0.01"
            step="0.01"
            class="th-input w-32"
            :placeholder="t('billingPage.yuanPh')"
          />
          <p v-if="useCustom && customQuotaEstimate != null" class="w-full text-xs th-billing-package__price">
            {{ t('billingPage.customQuotaEstimate', { n: customQuotaEstimate }) }}
          </p>
        </div>

        <div v-if="providers.length" class="mt-4">
          <label class="mb-1 block text-xs th-billing-panel__hint">{{ t('billingPage.provider') }}</label>
          <select v-model="provider" class="th-input w-full max-w-xs">
            <option v-for="pr in providers" :key="pr.id" :value="pr.id">{{ pr.label }}</option>
          </select>
        </div>
        <p v-else class="mt-4 text-sm text-amber-600">{{ t('billingPage.noProviders') }}</p>

        <div v-if="showEasypayType" class="mt-3">
          <label class="mb-1 block text-xs th-billing-panel__hint">{{ t('billingPage.easypayType') }}</label>
          <select v-model="easypayType" class="th-input w-full max-w-xs">
            <option v-for="opt in easypayTypeOptions" :key="opt.id" :value="opt.id">{{ opt.label }}</option>
          </select>
        </div>

        <button
          type="button"
          class="th-btn-primary mt-4 disabled:cursor-not-allowed disabled:opacity-50"
          :disabled="loading || !providers.length"
          @click="createOrder"
        >
          {{ loading ? t('billingPage.submitting') : t('billingPage.submit') }}
        </button>
        <p v-if="msg" class="mt-3 text-sm" :class="msgTone === 'error' ? 'text-red-400' : 'text-emerald-400'">{{ msg }}</p>
      </section>

      <section v-if="activeOrder" class="th-billing-pay-panel">
        <h2 class="th-billing-pay-panel__title">{{ t('billingPage.payTitle') }}</h2>
        <p class="th-billing-pay-panel__meta">{{ t('billingPage.orderNo') }}: {{ activeOrder.order_no }}</p>
        <p class="th-billing-pay-panel__meta">¥{{ fmtYuan(activeOrder.amount_cents) }} → {{ activeOrder.quota_amount }} {{ t('billingPage.quotaUnit') }}</p>
        <button
          v-if="payCheckoutUrl"
          type="button"
          class="th-btn-primary mt-4 text-sm"
          @click="openCheckout"
        >
          {{ t('billingPage.openPay') }}
        </button>
        <div v-if="payQrImageSrc && !qrImgFailed" class="th-billing-qr-box mt-4">
          <p class="mb-3 text-center text-xs th-billing-panel__hint">{{ t('billingPage.scanQr') }}</p>
          <img
            :src="payQrImageSrc"
            alt=""
            class="th-billing-qr-img mx-auto block"
            @error="qrImgFailed = true"
          />
        </div>
        <p v-else-if="payCheckoutUrl && qrImgFailed" class="mt-3 text-xs th-billing-panel__hint">
          {{ t('billingPage.qrLoadFail') }}
        </p>
        <button v-if="provider === 'mock'" type="button" class="th-btn-secondary mt-4 text-sm" @click="mockPay">
          {{ t('billingPage.mockPayBtn') }}
        </button>
        <RouterLink to="/console/payment-orders" class="th-link-accent mt-3 block text-sm hover:underline">{{ t('billingPage.viewOrders') }}</RouterLink>
      </section>

      <section class="th-console-panel rounded-2xl p-6">
        <h2 class="text-sm font-medium th-billing-package__name">{{ t('billingPage.redeemTitle') }}</h2>
        <p class="mt-1 text-sm th-billing-panel__hint">{{ t('billingPage.redeemDesc') }}</p>
        <RouterLink to="/console/redeem" class="th-link-accent mt-2 inline-block text-sm hover:underline">{{ t('billingPage.redeemLink') }}</RouterLink>
      </section>
    </template>
  </div>
</template>
