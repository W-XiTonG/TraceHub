<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import { apiErrorMessage, useAuthStore } from '../stores/auth'

const { t } = useI18n()

type CustomAlertConfig = {
  emails?: string[]
  dingtalk_webhook?: string
  feishu_webhook?: string
  wecom_webhook?: string
  dingtalk_sign_configured?: boolean
  feishu_sign_configured?: boolean
  wecom_sign_configured?: boolean
  generic_webhook_sign_configured?: boolean
}

type Me = {
  id: number
  email: string
  username?: string
  group_name?: string
  balance?: number
  webhook_url?: string
  ip_whitelist?: string[]
  balance_alert_threshold?: number
  currency_quota_per_yuan?: number
  custom_alert_config?: CustomAlertConfig
}

const auth = useAuthStore()
const me = ref<Me | null>(null)
const loadErr = ref('')
const bindOk = ref('')
const bindErr = ref('')
const balanceOk = ref('')
const balanceErr = ref('')
const customOk = ref('')
const customErr = ref('')
const ipOk = ref('')
const ipErr = ref('')
const pwdOk = ref('')
const pwdErr = ref('')

function clearAllSaveFeedback() {
  bindOk.value = ''
  bindErr.value = ''
  balanceOk.value = ''
  balanceErr.value = ''
  customOk.value = ''
  customErr.value = ''
  ipOk.value = ''
  ipErr.value = ''
  pwdOk.value = ''
  pwdErr.value = ''
}

const pwd = reactive({ old: '', new1: '', new2: '' })
const bind = reactive({ username: '', webhook_url: '' })
const ipText = ref('')
const balanceAlertThreshold = ref('0')

const customEmailsText = ref('')
const customDing = ref('')
const customFeishu = ref('')
const customWecom = ref('')

const bindGenericSign = ref('')
const clearGenericSign = ref(false)

const secretDing = ref('')
const secretFeishu = ref('')
const secretWecom = ref('')
const clearDingSign = ref(false)
const clearFeishuSign = ref(false)
const clearWecomSign = ref(false)

const currencyUnit = computed(() => {
  const u = me.value?.currency_quota_per_yuan
  return u != null && u > 0 ? u : 10000
})

const thresholdYuan = computed(() => {
  const raw = Math.trunc(Number(balanceAlertThreshold.value))
  if (Number.isNaN(raw) || raw <= 0) return null
  const y = raw / currencyUnit.value
  return (Math.round(y * 10000) / 10000).toFixed(4)
})

function applyCustomFromMe(m: Me) {
  const c = m.custom_alert_config
  customEmailsText.value = (c?.emails || []).join('\n')
  customDing.value = c?.dingtalk_webhook || ''
  customFeishu.value = c?.feishu_webhook || ''
  customWecom.value = c?.wecom_webhook || ''
}

function syncAuthFromMe() {
  if (!me.value) return
  const token = localStorage.getItem('tracehub_token') || ''
  auth.setSession(token, {
    id: me.value.id,
    email: me.value.email,
    username: me.value.username,
    balance: me.value.balance,
    group_name: me.value.group_name,
  })
}

async function load() {
  loadErr.value = ''
  try {
    const { data } = await http.get<Envelope<Me>>('/api/user/me')
    if (data.code !== 0) throw new Error(data.message || t('settingsPage.loadFail'))
    me.value = data.data
    bind.username = data.data.username || ''
    bind.webhook_url = data.data.webhook_url || ''
    ipText.value = (data.data.ip_whitelist || []).join('\n')
    balanceAlertThreshold.value = String(Math.trunc(Number(data.data.balance_alert_threshold ?? 0)))
    applyCustomFromMe(data.data)
    syncAuthFromMe()
  } catch (e: unknown) {
    loadErr.value = apiErrorMessage(e, t('settingsPage.loadFail'))
  }
}

async function saveBind() {
  clearAllSaveFeedback()
  try {
    const body: Record<string, unknown> = {
      username: bind.username.trim() || undefined,
      webhook_url: bind.webhook_url,
    }
    const sec: Record<string, string> = {}
    if (clearGenericSign.value) sec.generic_webhook = ''
    else if (bindGenericSign.value.trim()) sec.generic_webhook = bindGenericSign.value.trim()
    if (Object.keys(sec).length) body.custom_alert_secrets = sec

    const { data } = await http.put<Envelope<null>>('/api/user/bind', body)
    if (data.code !== 0) throw new Error(data.message || t('settingsPage.saveFail'))
    bindOk.value = t('settingsPage.bindOk')
    bindGenericSign.value = ''
    clearGenericSign.value = false
    await load()
  } catch (e: unknown) {
    bindErr.value = apiErrorMessage(e, t('settingsPage.saveFail'))
  }
}

async function saveBalanceAlert() {
  clearAllSaveFeedback()
  const n = Math.trunc(Number(balanceAlertThreshold.value))
  if (Number.isNaN(n) || n < 0) {
    balanceErr.value = t('settingsPage.saveFail')
    return
  }
  try {
    const { data } = await http.put<Envelope<null>>('/api/user/bind', {
      balance_alert_threshold: n,
    })
    if (data.code !== 0) throw new Error(data.message || t('settingsPage.saveFail'))
    balanceOk.value = t('settingsPage.balanceAlertOk')
    await load()
  } catch (e: unknown) {
    balanceErr.value = apiErrorMessage(e, t('settingsPage.saveFail'))
  }
}

function buildCustomPayload(): CustomAlertConfig {
  const emails = customEmailsText.value
    .split(/[\n,;]+/)
    .map((s) => s.trim())
    .filter(Boolean)
  return {
    emails: emails.length ? emails : [],
    dingtalk_webhook: customDing.value.trim(),
    feishu_webhook: customFeishu.value.trim(),
    wecom_webhook: customWecom.value.trim(),
  }
}

async function saveCustomAlert() {
  clearAllSaveFeedback()
  try {
    const payload = buildCustomPayload()
    const body: Record<string, unknown> = { custom_alert_config: payload }
    const sec: Record<string, string> = {}
    if (clearDingSign.value) sec.dingtalk = ''
    else if (secretDing.value.trim()) sec.dingtalk = secretDing.value.trim()
    if (clearFeishuSign.value) sec.feishu = ''
    else if (secretFeishu.value.trim()) sec.feishu = secretFeishu.value.trim()
    if (clearWecomSign.value) sec.wecom = ''
    else if (secretWecom.value.trim()) sec.wecom = secretWecom.value.trim()
    if (Object.keys(sec).length) body.custom_alert_secrets = sec

    const { data } = await http.put<Envelope<null>>('/api/user/bind', body)
    if (data.code !== 0) throw new Error(data.message || t('settingsPage.saveFail'))
    customOk.value = t('settingsPage.customAlertOk')
    secretDing.value = ''
    secretFeishu.value = ''
    secretWecom.value = ''
    clearDingSign.value = false
    clearFeishuSign.value = false
    clearWecomSign.value = false
    await load()
  } catch (e: unknown) {
    customErr.value = apiErrorMessage(e, t('settingsPage.saveFail'))
  }
}

async function clearCustomAlert() {
  clearAllSaveFeedback()
  try {
    const { data } = await http.put<Envelope<null>>('/api/user/bind', {
      custom_alert_config: null,
    })
    if (data.code !== 0) throw new Error(data.message || t('settingsPage.saveFail'))
    customOk.value = t('settingsPage.customAlertCleared')
    await load()
  } catch (e: unknown) {
    customErr.value = apiErrorMessage(e, t('settingsPage.saveFail'))
  }
}

async function saveIp() {
  clearAllSaveFeedback()
  const lines = ipText.value
    .split(/[\n,]+/)
    .map((s) => s.trim())
    .filter(Boolean)
  try {
    const { data } = await http.put<Envelope<null>>('/api/user/ip_whitelist', { ip_whitelist: lines })
    if (data.code !== 0) throw new Error(data.message || t('settingsPage.saveFail'))
    ipOk.value = t('settingsPage.ipOk')
  } catch (e: unknown) {
    ipErr.value = apiErrorMessage(e, t('settingsPage.saveFail'))
  }
}

async function changePwd() {
  clearAllSaveFeedback()
  if (pwd.new1 !== pwd.new2) {
    pwdErr.value = t('settingsPage.pwdMismatch')
    return
  }
  try {
    const { data } = await http.put<Envelope<null>>('/api/user/password', {
      old_password: pwd.old,
      new_password: pwd.new1,
    })
    if (data.code !== 0) throw new Error(data.message || t('settingsPage.pwdChangeFail'))
    pwdOk.value = t('settingsPage.pwdOk')
    pwd.old = ''
    pwd.new1 = ''
    pwd.new2 = ''
  } catch (e: unknown) {
    pwdErr.value = apiErrorMessage(e, t('settingsPage.pwdChangeFail'))
  }
}

onMounted(load)
</script>

<template>
  <div class="max-w-3xl space-y-10">
    <div>
      <h1 class="mb-2 text-2xl font-bold">{{ t('settingsPage.title') }}</h1>
      <p class="text-sm text-slate-500">{{ t('settingsPage.desc') }}</p>
    </div>
    <p v-if="loadErr" class="text-sm text-red-400">{{ loadErr }}</p>

    <section v-if="me" class="rounded-xl border border-slate-800 bg-slate-900/50 p-5">
      <h2 class="mb-3 text-sm font-semibold text-slate-300">{{ t('settingsPage.secInfo') }}</h2>
      <p class="text-sm"><span class="text-slate-500">{{ t('settingsPage.email') }}</span> {{ me.email }}</p>
      <p class="mt-1 text-sm"><span class="text-slate-500">{{ t('settingsPage.group') }}</span> {{ me.group_name || t('settingsPage.dash') }}</p>
    </section>

    <section class="rounded-xl border border-slate-800 bg-slate-900/50 p-5">
      <h2 class="mb-3 text-sm font-semibold text-slate-300">{{ t('settingsPage.secBind') }}</h2>
      <p class="mb-3 text-xs text-slate-500">{{ t('settingsPage.bindHint') }}</p>
      <label class="mb-3 block text-sm">
        <span class="text-slate-500">{{ t('settingsPage.displayName') }}</span>
        <input v-model="bind.username" class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-3 py-2" />
      </label>
      <label class="mb-3 block text-sm">
        <span class="text-slate-500">{{ t('settingsPage.webhook') }}</span>
        <input v-model="bind.webhook_url" type="url" :placeholder="t('settingsPage.webhookPh')" class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 text-sm" />
      </label>
      <p class="mb-2 text-xs text-slate-500">{{ t('settingsPage.bindSignHint') }}</p>
      <label class="mb-2 block text-sm">
        <span class="text-slate-500">{{ t('settingsPage.bindGenericSign') }}</span>
        <input
          v-model="bindGenericSign"
          type="password"
          autocomplete="new-password"
          class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs"
          :placeholder="t('settingsPage.bindGenericSignPh')"
        />
      </label>
      <label class="mb-3 flex items-center gap-2 text-xs text-slate-400">
        <input v-model="clearGenericSign" type="checkbox" class="rounded border-slate-600" />
        {{ t('settingsPage.bindClearGenericSign') }}
      </label>
      <button type="button" class="th-btn-primary-flat px-4 py-2 text-sm" @click="saveBind">{{ t('settingsPage.saveBind') }}</button>
      <p v-if="bindErr" class="mt-2 text-sm text-red-400">{{ bindErr }}</p>
      <p v-if="bindOk" class="mt-2 text-sm text-emerald-400">{{ bindOk }}</p>
    </section>

    <section class="rounded-xl border border-slate-800 bg-slate-900/50 p-5">
      <h2 class="mb-3 text-sm font-semibold text-slate-300">{{ t('settingsPage.secBalanceAlert') }}</h2>
      <p class="mb-3 text-xs text-slate-500">{{ t('settingsPage.balanceAlertHint') }}</p>
      <label class="mb-2 block text-sm">
        <span class="text-slate-500">{{ t('settingsPage.balanceAlertThreshold') }}</span>
        <input
          v-model="balanceAlertThreshold"
          inputmode="numeric"
          class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-sm"
        />
      </label>
      <p v-if="thresholdYuan != null" class="mb-3 text-xs text-slate-500">
        {{ t('settingsPage.balanceAlertYuanHint', { yuan: thresholdYuan, unit: currencyUnit }) }}
      </p>
      <button type="button" class="th-btn-primary-flat px-4 py-2 text-sm" @click="saveBalanceAlert">{{ t('settingsPage.balanceAlertSave') }}</button>
      <p v-if="balanceErr" class="mt-2 text-sm text-red-400">{{ balanceErr }}</p>
      <p v-if="balanceOk" class="mt-2 text-sm text-emerald-400">{{ balanceOk }}</p>
    </section>

    <section class="rounded-xl border border-slate-800 bg-slate-900/50 p-5">
      <h2 class="mb-3 text-sm font-semibold text-slate-300">{{ t('settingsPage.secCustomAlert') }}</h2>
      <p class="mb-3 text-xs text-slate-500">{{ t('settingsPage.customAlertHint') }}</p>
      <p class="mb-3 text-xs text-slate-600">{{ t('settingsPage.customAlertSignHint') }}</p>
      <label class="mb-3 block text-sm">
        <span class="text-slate-500">{{ t('settingsPage.customAlertEmails') }}</span>
        <textarea
          v-model="customEmailsText"
          rows="4"
          class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs"
          :placeholder="t('settingsPage.customAlertEmailsPh')"
        />
      </label>
      <label class="mb-2 block text-sm">
        <span class="text-slate-500">{{ t('settingsPage.customAlertDing') }}</span>
        <input
          v-model="customDing"
          type="url"
          class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 text-xs"
          :placeholder="t('settingsPage.customAlertWebhookPh')"
        />
      </label>
      <label class="mb-1 block text-sm">
        <span class="text-slate-500">{{ t('settingsPage.customAlertDingSign') }}</span>
        <input v-model="secretDing" type="password" autocomplete="new-password" class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs" :placeholder="t('settingsPage.customAlertSignPh')" />
      </label>
      <label class="mb-3 flex items-center gap-2 text-xs text-slate-400">
        <input v-model="clearDingSign" type="checkbox" class="rounded border-slate-600" />
        {{ t('settingsPage.customAlertClearSignDing') }}
      </label>
      <label class="mb-2 block text-sm">
        <span class="text-slate-500">{{ t('settingsPage.customAlertFeishu') }}</span>
        <input
          v-model="customFeishu"
          type="url"
          class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 text-xs"
          :placeholder="t('settingsPage.customAlertWebhookPh')"
        />
      </label>
      <label class="mb-1 block text-sm">
        <span class="text-slate-500">{{ t('settingsPage.customAlertFeishuSign') }}</span>
        <input v-model="secretFeishu" type="password" autocomplete="new-password" class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs" :placeholder="t('settingsPage.customAlertSignPh')" />
      </label>
      <label class="mb-3 flex items-center gap-2 text-xs text-slate-400">
        <input v-model="clearFeishuSign" type="checkbox" class="rounded border-slate-600" />
        {{ t('settingsPage.customAlertClearSignFeishu') }}
      </label>
      <label class="mb-2 block text-sm">
        <span class="text-slate-500">{{ t('settingsPage.customAlertWecom') }}</span>
        <input
          v-model="customWecom"
          type="url"
          class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 text-xs"
          :placeholder="t('settingsPage.customAlertWebhookPh')"
        />
      </label>
      <label class="mb-1 block text-sm">
        <span class="text-slate-500">{{ t('settingsPage.customAlertWecomSign') }}</span>
        <input v-model="secretWecom" type="password" autocomplete="new-password" class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs" :placeholder="t('settingsPage.customAlertSignPh')" />
      </label>
      <label class="mb-4 flex items-center gap-2 text-xs text-slate-400">
        <input v-model="clearWecomSign" type="checkbox" class="rounded border-slate-600" />
        {{ t('settingsPage.customAlertClearSignWecom') }}
      </label>
      <div class="flex flex-wrap gap-2">
        <button type="button" class="th-btn-primary-flat px-4 py-2 text-sm" @click="saveCustomAlert">{{ t('settingsPage.customAlertSave') }}</button>
        <button type="button" class="rounded-lg border border-slate-600 px-4 py-2 text-sm text-slate-300 hover:bg-slate-800" @click="clearCustomAlert">
          {{ t('settingsPage.customAlertClear') }}
        </button>
      </div>
      <p v-if="customErr" class="mt-2 text-sm text-red-400">{{ customErr }}</p>
      <p v-if="customOk" class="mt-2 text-sm text-emerald-400">{{ customOk }}</p>
    </section>

    <section class="rounded-xl border border-slate-800 bg-slate-900/50 p-5">
      <h2 class="mb-3 text-sm font-semibold text-slate-300">{{ t('settingsPage.secIp') }}</h2>
      <p class="mb-3 text-xs text-slate-500">{{ t('settingsPage.ipHint') }}</p>
      <textarea v-model="ipText" rows="5" class="w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs" :placeholder="t('settingsPage.ipPh')" />
      <button type="button" class="th-btn-primary-flat mt-3 px-4 py-2 text-sm" @click="saveIp">{{ t('settingsPage.saveIp') }}</button>
      <p v-if="ipErr" class="mt-2 text-sm text-red-400">{{ ipErr }}</p>
      <p v-if="ipOk" class="mt-2 text-sm text-emerald-400">{{ ipOk }}</p>
    </section>

    <section class="rounded-xl border border-slate-800 bg-slate-900/50 p-5">
      <h2 class="mb-3 text-sm font-semibold text-slate-300">{{ t('settingsPage.secPwd') }}</h2>
      <label class="mb-2 block text-sm">
        <span class="text-slate-500">{{ t('settingsPage.curPwd') }}</span>
        <input v-model="pwd.old" type="password" class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-3 py-2" />
      </label>
      <label class="mb-2 block text-sm">
        <span class="text-slate-500">{{ t('settingsPage.newPwd') }}</span>
        <input v-model="pwd.new1" type="password" class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-3 py-2" />
      </label>
      <label class="mb-3 block text-sm">
        <span class="text-slate-500">{{ t('settingsPage.newPwd2') }}</span>
        <input v-model="pwd.new2" type="password" class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-3 py-2" />
      </label>
      <button type="button" class="th-btn-primary-flat px-4 py-2 text-sm" @click="changePwd">{{ t('settingsPage.updatePwdBtn') }}</button>
      <p v-if="pwdErr" class="mt-2 text-sm text-red-400">{{ pwdErr }}</p>
      <p v-if="pwdOk" class="mt-2 text-sm text-emerald-400">{{ pwdOk }}</p>
    </section>
  </div>
</template>
