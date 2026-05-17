<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import MobileLabelValueCard from '../components/ui/MobileLabelValueCard.vue'
import MobileLabelValueRow from '../components/ui/MobileLabelValueRow.vue'

const { t } = useI18n()

type GroupInfo = {
  id: number
  name: string
  description?: string
  rate_multiplier: number
  rpm_limit: number
  default_model?: string
  subscription_type?: string
}

type RateRow = {
  id: number
  model_name: string
  input_rate: number
  output_rate: number
  enabled: boolean
}

const group = ref<GroupInfo | null>(null)
const rates = ref<RateRow[]>([])
const err = ref('')

onMounted(async () => {
  try {
    const { data } = await http.get<Envelope<{ group: GroupInfo | null; model_rates: RateRow[] }>>('/api/user/group')
    if (data.code !== 0) throw new Error(data.message || t('modelsPage.loadFail'))
    group.value = data.data.group
    rates.value = data.data.model_rates || []
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('modelsPage.loadFail')
  }
})
</script>

<template>
  <div class="min-w-0">
    <h1 class="mb-2 text-xl font-bold sm:text-2xl">{{ t('groupPage.title') }}</h1>
    <p class="mb-6 text-sm text-slate-500">{{ t('groupPage.desc') }}</p>
    <p v-if="err" class="mb-4 text-sm text-red-400">{{ err }}</p>

    <div v-if="group" class="mb-8 rounded-xl border border-slate-800 bg-slate-900/60 p-4 sm:p-6">
      <h2 class="text-lg font-semibold text-cyan-400">{{ group.name }}</h2>
      <p v-if="group.description" class="mt-2 text-sm text-slate-400">{{ group.description }}</p>
      <dl class="mt-4 grid gap-2 text-sm sm:grid-cols-2">
        <div><span class="text-slate-500">{{ t('groupPage.rateMul') }}</span> <span class="ml-2 font-mono">{{ group.rate_multiplier }}×</span></div>
        <div><span class="text-slate-500">{{ t('groupPage.rpm') }}</span> <span class="ml-2 font-mono">{{ group.rpm_limit || t('modelsPage.dash') }}</span></div>
        <div v-if="group.default_model"><span class="text-slate-500">{{ t('groupPage.defModel') }}</span> <span class="ml-2 break-all font-mono text-xs">{{ group.default_model }}</span></div>
        <div><span class="text-slate-500">{{ t('groupPage.billType') }}</span> <span class="ml-2">{{ group.subscription_type || t('modelsPage.dash') }}</span></div>
      </dl>
    </div>
    <div v-else-if="!err" class="rounded-xl border border-slate-800 bg-slate-900/40 p-6 text-slate-500">{{ t('groupPage.noGroup') }}</div>

    <h3 class="mb-3 text-sm font-medium uppercase tracking-wide text-slate-500">{{ t('groupPage.ratesTitle') }}</h3>
    <div class="hidden overflow-x-auto rounded-xl border border-slate-800 lg:block">
      <table class="w-full text-left text-sm">
        <thead class="bg-slate-900 text-slate-500">
          <tr>
            <th class="px-3 py-2">{{ t('groupPage.thModel') }}</th>
            <th class="px-3 py-2">{{ t('groupPage.thIn') }}</th>
            <th class="px-3 py-2">{{ t('groupPage.thOut') }}</th>
            <th class="px-3 py-2">{{ t('groupPage.thEn') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800">
          <tr v-for="r in rates" :key="r.id">
            <td class="px-3 py-2 font-mono text-xs">{{ r.model_name }}</td>
            <td class="px-3 py-2">{{ r.input_rate }}</td>
            <td class="px-3 py-2">{{ r.output_rate }}</td>
            <td class="px-3 py-2">{{ r.enabled ? t('groupPage.yes') : t('groupPage.no') }}</td>
          </tr>
          <tr v-if="!rates.length">
            <td colspan="4" class="px-3 py-6 text-center text-slate-500">{{ t('groupPage.noRates') }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="mt-3 space-y-3 lg:hidden">
      <MobileLabelValueCard v-for="r in rates" :key="`gr-m-${r.id}`" :title="r.model_name">
        <MobileLabelValueRow :label="t('groupPage.thIn')">{{ r.input_rate }}</MobileLabelValueRow>
        <MobileLabelValueRow :label="t('groupPage.thOut')">{{ r.output_rate }}</MobileLabelValueRow>
        <MobileLabelValueRow :label="t('groupPage.thEn')">{{ r.enabled ? t('groupPage.yes') : t('groupPage.no') }}</MobileLabelValueRow>
      </MobileLabelValueCard>
      <p v-if="!rates.length" class="py-6 text-center text-sm text-slate-500">{{ t('groupPage.noRates') }}</p>
    </div>
  </div>
</template>
