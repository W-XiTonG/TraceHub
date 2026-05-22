<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import { apiErrorMessage } from '../stores/auth'
import MobileLabelValueCard from '../components/ui/MobileLabelValueCard.vue'
import MobileLabelValueRow from '../components/ui/MobileLabelValueRow.vue'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import ThTableSkeleton from '../components/ui/ThTableSkeleton.vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'

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
const syncBusy = ref<number | null>(null)
const { busy: loading, visible: loadingUi, run: runLoad } = useSmoothLoading()

async function load() {
  await runLoad(async () => {
    const { data } = await http.get<Envelope<{ items: Row[] }>>('/api/admin/payment/orders', { params: { page_size: 50 } })
    if (data.code === 0 && data.data) items.value = data.data.items || []
  })
}

async function refund(id: number) {
  if (!confirm(t('adminPayment.refundConfirm'))) return
  await http.post(`/api/admin/payment/orders/${id}/refund`, {})
  await load()
}

async function syncOrder(id: number) {
  syncBusy.value = id
  try {
    const { data } = await http.post<Envelope<{ status: string }>>(`/api/admin/payment/orders/${id}/sync`, {})
    if (data.code !== 0) {
      alert(apiErrorMessage(data))
      return
    }
    await load()
  } catch (e: unknown) {
    alert(apiErrorMessage(e))
  } finally {
    syncBusy.value = null
  }
}

function canSync(r: Row) {
  return r.status === 'pending' && r.provider === 'easypay'
}

onMounted(() => void load())
</script>

<template>
  <div class="max-w-5xl min-w-0">
    <h1 class="mb-4 text-xl font-bold sm:text-2xl">{{ t('adminPayment.ordersTitle') }}</h1>
    <ThTableSkeleton v-if="loadingUi && items.length === 0" :rows="6" :columns="7" />
    <div v-if="loadingUi && items.length === 0" class="py-8 lg:hidden">
      <ThLoadingState />
    </div>
    <template v-if="!loadingUi || items.length > 0">
      <div class="relative hidden overflow-x-auto rounded-xl border border-slate-800 lg:block">
        <ThLoadingState v-if="loadingUi" variant="overlay" />
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
              <td class="px-3 py-2 space-x-2">
                <button
                  v-if="canSync(r)"
                  type="button"
                  class="text-xs text-cyan-400 hover:underline disabled:opacity-40"
                  :disabled="syncBusy === r.id"
                  @click="syncOrder(r.id)"
                >
                  {{ t('adminPayment.sync') }}
                </button>
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
          <div class="flex gap-3 pt-1">
            <button
              v-if="canSync(r)"
              type="button"
              class="text-xs text-cyan-400 hover:underline disabled:opacity-40"
              :disabled="syncBusy === r.id"
              @click="syncOrder(r.id)"
            >
              {{ t('adminPayment.sync') }}
            </button>
            <button v-if="r.status === 'paid'" type="button" class="text-xs text-amber-400 hover:underline" @click="refund(r.id)">
              {{ t('adminPayment.refund') }}
            </button>
          </div>
        </MobileLabelValueCard>
      </div>
    </template>
  </div>
</template>
