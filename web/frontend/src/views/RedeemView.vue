<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import ContactCardDisclosure from '../components/ContactCardDisclosure.vue'

const { t } = useI18n()

const code = ref('')
const msg = ref('')
const result = ref<Record<string, unknown> | null>(null)

async function redeem() {
  msg.value = ''
  result.value = null
  try {
    const { data } = await http.post<Envelope<Record<string, unknown>>>('/api/user/redeem', { code: code.value })
    if (data.code !== 0) throw new Error(data.message || t('redeemPage.fail'))
    msg.value = data.message || t('redeemPage.success')
    result.value = data.data
  } catch (e: unknown) {
    msg.value = e instanceof Error ? e.message : t('redeemPage.fail')
  }
}
</script>

<template>
  <div class="max-w-md">
    <h1 class="mb-6 text-2xl font-bold">{{ t('redeemPage.title') }}</h1>
    <input v-model="code" :placeholder="t('redeemPage.ph')" class="th-input mb-4 w-full" />
    <button type="button" class="th-btn-primary-flat w-full py-2.5 text-sm" @click="redeem">{{ t('redeemPage.submit') }}</button>
    <p class="mt-4 text-sm text-slate-400">{{ msg }}</p>
    <pre v-if="result" class="mt-4 overflow-auto rounded-lg bg-slate-900 p-4 text-xs">{{ JSON.stringify(result, null, 2) }}</pre>
    <div class="mt-8 border-t border-white/[0.06] pt-6">
      <ContactCardDisclosure align="start" compact summary-key="home.contactHintRedeem" />
    </div>
  </div>
</template>
