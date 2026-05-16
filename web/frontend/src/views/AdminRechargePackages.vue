<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'

const { t } = useI18n()

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
const form = ref({ name: '', amount_cents: 1000, quota_amount: 100000, bonus_quota: 0, currency: 'CNY', enabled: true, sort: 0 })

async function load() {
  const { data } = await http.get<Envelope<{ items: Pkg[] }>>('/api/admin/payment/packages')
  if (data.code === 0 && data.data) items.value = data.data.items || []
}

async function create() {
  await http.post('/api/admin/payment/packages', form.value)
  form.value.name = ''
  await load()
}

async function remove(id: number) {
  if (!confirm(t('adminPayment.deleteConfirm'))) return
  await http.delete(`/api/admin/payment/packages/${id}`)
  await load()
}

onMounted(() => void load())
</script>

<template>
  <div class="max-w-3xl space-y-6">
    <h1 class="text-2xl font-bold">{{ t('adminPayment.packagesTitle') }}</h1>
    <div class="rounded-xl border border-slate-800 p-4 space-y-2">
      <input v-model="form.name" class="th-input w-full" :placeholder="t('adminPayment.pkgName')" />
      <div class="grid grid-cols-2 gap-2">
        <input v-model.number="form.amount_cents" type="number" class="th-input" placeholder="amount_cents" />
        <input v-model.number="form.quota_amount" type="number" class="th-input" placeholder="quota_amount" />
      </div>
      <button type="button" class="th-btn-primary text-sm" @click="create">{{ t('adminPayment.addPkg') }}</button>
    </div>
    <ul class="space-y-2">
      <li v-for="p in items" :key="p.id" class="flex items-center justify-between rounded-lg border border-slate-800 px-4 py-3 text-sm">
        <span>{{ p.name }} · ¥{{ (p.amount_cents / 100).toFixed(2) }} → {{ p.quota_amount }}</span>
        <button type="button" class="text-red-400 text-xs" @click="remove(p.id)">{{ t('adminPayment.delete') }}</button>
      </li>
    </ul>
  </div>
</template>
