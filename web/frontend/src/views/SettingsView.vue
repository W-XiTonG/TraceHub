<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import { useAuthStore } from '../stores/auth'

const { t } = useI18n()

type Me = {
  id: number
  email: string
  username?: string
  group_name?: string
  balance?: number
  webhook_url?: string
  ip_whitelist?: string[]
}

const auth = useAuthStore()
const me = ref<Me | null>(null)
const err = ref('')
const ok = ref('')

const pwd = reactive({ old: '', new1: '', new2: '' })
const bind = reactive({ username: '', webhook_url: '' })
const ipText = ref('')

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
  err.value = ''
  try {
    const { data } = await http.get<Envelope<Me>>('/api/user/me')
    if (data.code !== 0) throw new Error(data.message || t('settingsPage.loadFail'))
    me.value = data.data
    bind.username = data.data.username || ''
    bind.webhook_url = data.data.webhook_url || ''
    ipText.value = (data.data.ip_whitelist || []).join('\n')
    syncAuthFromMe()
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('settingsPage.loadFail')
  }
}

async function saveBind() {
  ok.value = ''
  err.value = ''
  try {
    const { data } = await http.put<Envelope<null>>('/api/user/bind', {
      username: bind.username.trim() || undefined,
      webhook_url: bind.webhook_url,
    })
    if (data.code !== 0) throw new Error(data.message || t('settingsPage.saveFail'))
    ok.value = t('settingsPage.bindOk')
    await load()
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('settingsPage.saveFail')
  }
}

async function saveIp() {
  ok.value = ''
  err.value = ''
  const lines = ipText.value
    .split(/[\n,]+/)
    .map((s) => s.trim())
    .filter(Boolean)
  try {
    const { data } = await http.put<Envelope<null>>('/api/user/ip_whitelist', { ip_whitelist: lines })
    if (data.code !== 0) throw new Error(data.message || t('settingsPage.saveFail'))
    ok.value = t('settingsPage.ipOk')
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('settingsPage.saveFail')
  }
}

async function changePwd() {
  ok.value = ''
  err.value = ''
  if (pwd.new1 !== pwd.new2) {
    err.value = t('settingsPage.pwdMismatch')
    return
  }
  try {
    const { data } = await http.put<Envelope<null>>('/api/user/password', {
      old_password: pwd.old,
      new_password: pwd.new1,
    })
    if (data.code !== 0) throw new Error(data.message || t('settingsPage.pwdChangeFail'))
    ok.value = t('settingsPage.pwdOk')
    pwd.old = ''
    pwd.new1 = ''
    pwd.new2 = ''
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('settingsPage.pwdChangeFail')
  }
}

onMounted(load)
</script>

<template>
  <div class="max-w-xl space-y-10">
    <div>
      <h1 class="mb-2 text-2xl font-bold">{{ t('settingsPage.title') }}</h1>
      <p class="text-sm text-slate-500">{{ t('settingsPage.desc') }}</p>
    </div>
    <p v-if="ok" class="text-sm text-emerald-400">{{ ok }}</p>
    <p v-if="err" class="text-sm text-red-400">{{ err }}</p>

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
      <button type="button" class="th-btn-primary-flat px-4 py-2 text-sm" @click="saveBind">{{ t('settingsPage.saveBind') }}</button>
    </section>

    <section class="rounded-xl border border-slate-800 bg-slate-900/50 p-5">
      <h2 class="mb-3 text-sm font-semibold text-slate-300">{{ t('settingsPage.secIp') }}</h2>
      <p class="mb-3 text-xs text-slate-500">{{ t('settingsPage.ipHint') }}</p>
      <textarea v-model="ipText" rows="5" class="w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs" :placeholder="t('settingsPage.ipPh')" />
      <button type="button" class="th-btn-primary-flat mt-3 px-4 py-2 text-sm" @click="saveIp">{{ t('settingsPage.saveIp') }}</button>
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
      <button type="button" class="rounded-lg bg-slate-700 px-4 py-2 text-sm hover:bg-slate-600" @click="changePwd">{{ t('settingsPage.updatePwdBtn') }}</button>
    </section>
  </div>
</template>
