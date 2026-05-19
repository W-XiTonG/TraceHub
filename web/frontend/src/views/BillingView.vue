<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import { apiErrorMessage } from '../stores/auth'
import contactCardImg from '../assets/xunji-contact-card.png'
import { useSiteMetaStore } from '../stores/siteMeta'

const { t } = useI18n()
const route = useRoute()
const siteMeta = useSiteMetaStore()

const onlinePay = computed(() => siteMeta.onlinePaymentEnabled)

type Package = {
  id: number
  name: string
  amount_cents: number
  quota_amount: number
  bonus_quota?: number
  currency: string
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
const selectedPkg = ref<number | null>(null)
const provider = ref('mock')
const customYuan = ref('')
const useCustom = ref(false)
const loading = ref(false)
const msg = ref('')
const msgTone = ref<'success' | 'error'>('success')
const activeOrder = ref<OrderResult | null>(null)
let pollTimer: ReturnType<typeof setInterval> | null = null

const providers = [
  { id: 'mock', label: 'Mock（开发）' },
  { id: 'alipay', label: '支付宝' },
  { id: 'wxpay', label: '微信支付' },
  { id: 'stripe', label: 'Stripe' },
  { id: 'easypay', label: 'EasyPay' },
]

function fmtYuan(cents: number) {
  return (cents / 100).toFixed(2)
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
    const body: Record<string, unknown> = { provider: provider.value }
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
    activeOrder.value = data.data
    msgTone.value = 'success'
    msg.value = t('billingPage.orderCreated')
    startPoll(data.data.order_no)
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

onMounted(async () => {
  await siteMeta.ensureLoaded()
  if (onlinePay.value) {
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
    <p class="mb-6 text-sm text-slate-500">
      {{ onlinePay ? t('billingPage.descOnline') : t('billingPage.desc') }}
    </p>

    <!-- 在线支付关闭：原兑换码 + 企微 + 对公说明 -->
    <template v-if="!onlinePay">
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

      <div class="space-y-4 rounded-xl border border-slate-800 bg-slate-900/60 p-6">
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
      <section class="mb-8 rounded-xl border border-slate-800 bg-slate-900/60 p-6">
        <h2 class="mb-4 text-base font-semibold text-slate-200">{{ t('billingPage.packagesTitle') }}</h2>
        <div v-if="packages.length" class="grid gap-3 sm:grid-cols-2">
          <label
            v-for="p in packages"
            :key="p.id"
            class="cursor-pointer rounded-lg border p-4 transition"
            :class="!useCustom && selectedPkg === p.id ? 'border-cyan-500/60 bg-cyan-950/30' : 'border-slate-700 hover:border-slate-600'"
          >
            <input v-model="selectedPkg" type="radio" class="sr-only" :value="p.id" @change="useCustom = false" />
            <p class="font-medium text-slate-100">{{ p.name }}</p>
            <p class="mt-1 text-sm text-cyan-400">¥{{ fmtYuan(p.amount_cents) }}</p>
            <p class="mt-1 text-xs text-slate-500">{{ t('billingPage.quotaLabel', { n: p.quota_amount + (p.bonus_quota || 0) }) }}</p>
          </label>
        </div>
        <p v-else class="text-sm text-slate-500">{{ t('billingPage.noPackages') }}</p>

        <div class="mt-4 flex flex-wrap items-center gap-3 border-t border-slate-800 pt-4">
          <label class="flex items-center gap-2 text-sm text-slate-300">
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
        </div>

        <div class="mt-4">
          <label class="mb-1 block text-xs text-slate-500">{{ t('billingPage.provider') }}</label>
          <select v-model="provider" class="th-input w-full max-w-xs">
            <option v-for="pr in providers" :key="pr.id" :value="pr.id">{{ pr.label }}</option>
          </select>
        </div>

        <button type="button" class="th-btn-primary mt-4" :disabled="loading" @click="createOrder">
          {{ loading ? t('billingPage.submitting') : t('billingPage.submit') }}
        </button>
        <p v-if="msg" class="mt-3 text-sm" :class="msgTone === 'error' ? 'text-red-400' : 'text-emerald-400'">{{ msg }}</p>
      </section>

      <section v-if="activeOrder" class="mb-8 rounded-xl border border-cyan-500/30 bg-cyan-950/20 p-6">
        <h2 class="text-base font-semibold text-cyan-100">{{ t('billingPage.payTitle') }}</h2>
        <p class="mt-1 text-sm text-slate-400">{{ t('billingPage.orderNo') }}: {{ activeOrder.order_no }}</p>
        <p class="text-sm text-slate-400">¥{{ fmtYuan(activeOrder.amount_cents) }} → {{ activeOrder.quota_amount }} {{ t('billingPage.quotaUnit') }}</p>
        <a
          v-if="activeOrder.pay_url"
          :href="activeOrder.pay_url"
          target="_blank"
          rel="noopener"
          class="mt-3 inline-block text-sm text-cyan-400 hover:underline"
        >{{ t('billingPage.openPay') }}</a>
        <p v-if="activeOrder.qr_code" class="mt-2 break-all text-xs text-slate-500">{{ activeOrder.qr_code }}</p>
        <button v-if="provider === 'mock'" type="button" class="th-btn-secondary mt-4 text-sm" @click="mockPay">
          {{ t('billingPage.mockPayBtn') }}
        </button>
        <RouterLink to="/console/payment-orders" class="mt-3 block text-sm text-slate-500 hover:text-cyan-400">{{ t('billingPage.viewOrders') }}</RouterLink>
      </section>

      <section class="rounded-2xl border border-slate-800 bg-slate-900/40 p-6">
        <h2 class="text-sm font-medium text-slate-300">{{ t('billingPage.redeemTitle') }}</h2>
        <p class="mt-1 text-sm text-slate-500">{{ t('billingPage.redeemDesc') }}</p>
        <RouterLink to="/console/redeem" class="mt-2 inline-block text-sm text-cyan-400 hover:underline">{{ t('billingPage.redeemLink') }}</RouterLink>
      </section>
    </template>
  </div>
</template>
