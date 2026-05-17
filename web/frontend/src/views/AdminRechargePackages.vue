<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AdminActionBanner from '../components/admin/AdminActionBanner.vue'
import { useAdminActionFeedback } from '../composables/useAdminActionFeedback'
import { http, type Envelope } from '../api/http'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'

const { t } = useI18n()
const { flashMsg, flashIsError, busy, runAction, flashScrollId } = useAdminActionFeedback()

type Pkg = {
  id: number
  name: string
  amount_cents: number
  quota_amount: number
  bonus_quota: number
  currency: string
  enabled: boolean
  sort: number
}

const items = ref<Pkg[]>([])
const { busy: listLoading, visible: listLoadingUi, run: runListLoad } = useSmoothLoading()
const form = ref({ name: '', amount_cents: 1000, quota_amount: 100000, bonus_quota: 0, currency: 'CNY', enabled: true, sort: 0 })

async function load() {
  await runListLoad(async () => {
    const { data } = await http.get<Envelope<{ items: Pkg[] }>>('/api/admin/payment/packages')
    if (data.code === 0 && data.data) items.value = data.data.items || []
  })
}

async function create() {
  await runAction({
    validate: () => {
      if (!form.value.name.trim()) return t('adminPayment.pkgName')
      return null
    },
    successMessage: t('adminCommon.created'),
    action: async () => {
      const { data } = await http.post<Envelope<unknown>>('/api/admin/payment/packages', form.value)
      if (data.code !== 0) throw new Error(data.message || t('adminCommon.requestFail'))
      form.value.name = ''
    },
    onSuccess: () => load(),
  })
}

async function remove(id: number) {
  if (!confirm(t('adminPayment.deleteConfirm'))) return
  await runAction({
    successMessage: t('adminCommon.deleted'),
    action: async () => {
      const { data } = await http.delete<Envelope<unknown>>(`/api/admin/payment/packages/${id}`)
      if (data && typeof data === 'object' && 'code' in data && data.code !== 0) {
        throw new Error(data.message || t('adminCommon.requestFail'))
      }
    },
    onSuccess: () => load(),
  })
}

onMounted(() => void load())
</script>

<template>
  <div class="max-w-3xl space-y-6">
    <h1 class="text-2xl font-bold">{{ t('adminPayment.packagesTitle') }}</h1>
    <AdminActionBanner
      :message="flashMsg"
      :is-error="flashIsError"
      :loading="busy"
      :scroll-id="flashScrollId"
    />
    <div class="rounded-xl border border-slate-800 p-4 space-y-2">
      <input v-model="form.name" class="th-input w-full" :placeholder="t('adminPayment.pkgName')" />
      <div class="grid grid-cols-2 gap-2">
        <input v-model.number="form.amount_cents" type="number" class="th-input" placeholder="amount_cents" />
        <input v-model.number="form.quota_amount" type="number" class="th-input" placeholder="quota_amount" />
      </div>
      <button type="button" class="th-btn-primary text-sm disabled:opacity-50" :disabled="busy" @click="create">
        {{ busy ? t('adminCommon.saving') : t('adminPayment.addPkg') }}
      </button>
    </div>
    <ThLoadingState v-if="listLoadingUi && !items.length" class="py-8" />
    <ul v-else class="space-y-2">
      <li v-for="p in items" :key="p.id" class="flex items-center justify-between rounded-lg border border-slate-800 px-4 py-3 text-sm">
        <span>{{ p.name }} · ¥{{ (p.amount_cents / 100).toFixed(2) }} → {{ p.quota_amount }}</span>
        <button type="button" class="text-red-400 text-xs disabled:opacity-40" :disabled="busy" @click="remove(p.id)">{{ t('adminPayment.delete') }}</button>
      </li>
    </ul>
  </div>
</template>
