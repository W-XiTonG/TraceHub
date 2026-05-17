<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import MobileLabelValueCard from '../components/ui/MobileLabelValueCard.vue'
import MobileLabelValueRow from '../components/ui/MobileLabelValueRow.vue'

const { t } = useI18n()

type Row = {
  id: number
  order_no: string
  user_id: number
  provider: string
  amount_cents: number
  status: string
  created_at: string
}

const items = ref<Row[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const { data } = await http.get<Envelope<{ items: Row[] }>>('/api/admin/payment/orders', { params: { page_size: 50 } })
    if (data.code === 0 && data.data) items.value = data.data.items || []
  } finally {
    loading.value = false
  }
}

async function refund(id: number) {
  if (!confirm(t('adminPayment.refundConfirm'))) return
  await http.post(`/api/admin/payment/orders/${id}/refund`, {})
  await load()
}

onMounted(() => void load())
</script>

<template>
  <div class="max-w-5xl min-w-0">
    <h1 class="mb-4 text-xl font-bold sm:text-2xl">{{ t('adminPayment.ordersTitle') }}</h1>
    <div v-if="loading" class="text-sm text-slate-500">{{ t('common.loading') }}</div>
    <template v-else>
      <div class="hidden overflow-x-auto rounded-xl border border-slate-800 lg:block">
        <table class="w-full text-left text-sm">
          <thead class="border-b border-slate-800 text-slate-400">
            <tr>
              <th class="px-3 py-2">ID</th>
              <th class="px-3 py-2">{{ t('adminPayment.thOrder') }}</th>
              <th class="px-3 py-2">{{ t('adminPayment.thUser') }}</th>
              <th class="px-3 py-2">{{ t('adminPayment.thProvider') }}</th>
              <th class="px-3 py-2">{{ t('adminPayment.thAmount') }}</th>
              <th class="px-3 py-2">{{ t('adminPayment.thStatus') }}</th>
              <th class="px-3 py-2"></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in items" :key="r.id" class="border-b border-slate-800/50">
              <td class="px-3 py-2">{{ r.id }}</td>
              <td class="px-3 py-2 font-mono text-xs">{{ r.order_no }}</td>
              <td class="px-3 py-2">{{ r.user_id }}</td>
              <td class="px-3 py-2">{{ r.provider }}</td>
              <td class="px-3 py-2">{{ (r.amount_cents / 100).toFixed(2) }}</td>
              <td class="px-3 py-2">{{ r.status }}</td>
              <td class="px-3 py-2">
                <button v-if="r.status === 'paid'" type="button" class="text-xs text-amber-400 hover:underline" @click="refund(r.id)">
                  {{ t('adminPayment.refund') }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="space-y-3 lg:hidden">
        <MobileLabelValueCard v-for="r in items" :key="`apo-${r.id}`" :title="r.order_no">
          <MobileLabelValueRow label="ID">{{ r.id }}</MobileLabelValueRow>
          <MobileLabelValueRow :label="t('adminPayment.thUser')">{{ r.user_id }}</MobileLabelValueRow>
          <MobileLabelValueRow :label="t('adminPayment.thProvider')">{{ r.provider }}</MobileLabelValueRow>
          <MobileLabelValueRow :label="t('adminPayment.thAmount')">{{ (r.amount_cents / 100).toFixed(2) }}</MobileLabelValueRow>
          <MobileLabelValueRow :label="t('adminPayment.thStatus')">{{ r.status }}</MobileLabelValueRow>
          <div v-if="r.status === 'paid'" class="pt-1">
            <button type="button" class="text-xs text-amber-400 hover:underline" @click="refund(r.id)">{{ t('adminPayment.refund') }}</button>
          </div>
        </MobileLabelValueCard>
      </div>
    </template>
  </div>
</template>
