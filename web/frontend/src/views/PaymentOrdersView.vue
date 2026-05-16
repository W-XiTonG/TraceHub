<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'

const { t } = useI18n()

type Row = {
  id: number
  order_no: string
  provider: string
  amount_cents: number
  quota_amount: number
  status: string
  created_at: string
  paid_at?: string
}

const items = ref<Row[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)

function fmtYuan(c: number) {
  return (c / 100).toFixed(2)
}

function fmtTime(iso?: string) {
  if (!iso) return '—'
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

async function load() {
  loading.value = true
  try {
    const { data } = await http.get<Envelope<{ items: Row[]; total: number }>>('/api/user/payment/orders', {
      params: { page: page.value, page_size: 20 },
    })
    if (data.code === 0 && data.data) {
      items.value = data.data.items || []
      total.value = data.data.total || 0
    }
  } finally {
    loading.value = false
  }
}

onMounted(() => void load())
</script>

<template>
  <div class="max-w-4xl">
    <h1 class="mb-2 text-2xl font-bold">{{ t('paymentOrders.title') }}</h1>
    <p class="mb-6 text-sm text-slate-500">{{ t('paymentOrders.desc') }}</p>
    <div v-if="loading" class="text-sm text-slate-500">{{ t('paymentOrders.loading') }}</div>
    <div v-else class="overflow-x-auto rounded-xl border border-slate-800">
      <table class="w-full text-left text-sm">
        <thead class="border-b border-slate-800 bg-slate-900/80 text-slate-400">
          <tr>
            <th class="px-4 py-3">{{ t('paymentOrders.thOrder') }}</th>
            <th class="px-4 py-3">{{ t('paymentOrders.thProvider') }}</th>
            <th class="px-4 py-3">{{ t('paymentOrders.thAmount') }}</th>
            <th class="px-4 py-3">{{ t('paymentOrders.thQuota') }}</th>
            <th class="px-4 py-3">{{ t('paymentOrders.thStatus') }}</th>
            <th class="px-4 py-3">{{ t('paymentOrders.thTime') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in items" :key="r.id" class="border-b border-slate-800/60">
            <td class="px-4 py-3 font-mono text-xs">{{ r.order_no }}</td>
            <td class="px-4 py-3">{{ r.provider }}</td>
            <td class="px-4 py-3">¥{{ fmtYuan(r.amount_cents) }}</td>
            <td class="px-4 py-3">{{ r.quota_amount }}</td>
            <td class="px-4 py-3">{{ r.status }}</td>
            <td class="px-4 py-3 text-slate-500">{{ fmtTime(r.paid_at || r.created_at) }}</td>
          </tr>
        </tbody>
      </table>
      <p v-if="!items.length" class="p-6 text-center text-slate-500">{{ t('paymentOrders.empty') }}</p>
    </div>
    <p class="mt-4 text-xs text-slate-600">{{ t('paymentOrders.total', { n: total }) }}</p>
  </div>
</template>
