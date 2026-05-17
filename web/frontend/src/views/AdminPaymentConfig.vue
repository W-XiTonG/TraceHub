<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import { apiErrorMessage } from '../stores/auth'
import { useSiteMetaStore } from '../stores/siteMeta'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'

const { t } = useI18n()
const siteMeta = useSiteMetaStore()

const cfg = ref<Record<string, unknown>>({})
const { busy: pageLoading, visible: pageLoadingUi, run: runPageLoad } = useSmoothLoading()
const saving = ref(false)
const msg = ref('')

async function load() {
  await runPageLoad(async () => {
    const { data } = await http.get<Envelope<Record<string, unknown>>>('/api/admin/payment/config')
    if (data.code === 0 && data.data) cfg.value = { ...data.data }
  })
}

async function save() {
  saving.value = true
  msg.value = ''
  try {
    const { data } = await http.put<Envelope<unknown>>('/api/admin/payment/config', cfg.value)
    if (data.code === 0) {
      msg.value = t('adminPayment.saved')
      if (data.data && typeof data.data === 'object') cfg.value = data.data as Record<string, unknown>
      await siteMeta.refreshFromServer()
    } else {
      msg.value = apiErrorMessage(data)
    }
  } catch (e: unknown) {
    msg.value = apiErrorMessage(e)
  } finally {
    saving.value = false
  }
}

onMounted(() => void load())
</script>

<template>
  <div class="max-w-2xl space-y-6">
    <h1 class="text-2xl font-bold">{{ t('adminPayment.configTitle') }}</h1>
    <p class="text-sm text-slate-500">{{ t('adminPayment.configHint') }}</p>
    <ThLoadingState v-if="pageLoadingUi" class="py-10" />

    <template v-else>
    <div class="space-y-2 rounded-xl border border-amber-500/30 bg-amber-950/20 p-6">
      <label class="flex items-center gap-3 text-sm font-medium text-amber-100">
        <input v-model="cfg.online_enabled" type="checkbox" class="rounded" />
        {{ t('adminPayment.onlineEnabled') }}
      </label>
      <p class="text-xs text-slate-500">{{ t('adminPayment.onlineEnabledHint') }}</p>
    </div>
    <div class="space-y-4 rounded-xl border border-slate-800 bg-slate-900/60 p-6" :class="{ 'opacity-50 pointer-events-none': !cfg.online_enabled }">
      <label class="flex items-center gap-2 text-sm">
        <input v-model="cfg.mock_enabled" type="checkbox" />
        Mock
      </label>
      <label class="flex items-center gap-2 text-sm">
        <input v-model="cfg.alipay_enabled" type="checkbox" />
        {{ t('adminPayment.alipay') }}
      </label>
      <input v-model="cfg.alipay_app_id" class="th-input w-full" placeholder="Alipay App ID" />
      <label class="flex items-center gap-2 text-sm">
        <input v-model="cfg.wxpay_enabled" type="checkbox" />
        {{ t('adminPayment.wxpay') }}
      </label>
      <input v-model="cfg.wxpay_mch_id" class="th-input w-full" placeholder="WxPay Mch ID" />
      <label class="flex items-center gap-2 text-sm">
        <input v-model="cfg.stripe_enabled" type="checkbox" />
        Stripe
      </label>
      <input v-model="cfg.stripe_publishable_key" class="th-input w-full" placeholder="Stripe Publishable Key" />
      <input v-model="cfg.stripe_secret_key" class="th-input w-full" placeholder="Stripe Secret Key" type="password" />
      <label class="flex items-center gap-2 text-sm">
        <input v-model="cfg.easypay_enabled" type="checkbox" />
        EasyPay
      </label>
      <input v-model="cfg.easypay_pid" class="th-input w-full" placeholder="EasyPay PID" />
      <input v-model="cfg.site_base_url" class="th-input w-full" :placeholder="t('adminPayment.siteBase')" />
      <button type="button" class="th-btn-primary disabled:cursor-not-allowed disabled:opacity-50" :disabled="saving" @click="save">
        {{ saving ? t('adminCommon.saving') : t('adminPayment.save') }}
      </button>
      <p v-if="msg" class="text-sm text-emerald-400">{{ msg }}</p>
    </div>
    </template>
  </div>
</template>
