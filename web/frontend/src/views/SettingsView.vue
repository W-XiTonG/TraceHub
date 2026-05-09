<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { http, type Envelope } from '../api/http'
import { useAuthStore } from '../stores/auth'

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
  const t = localStorage.getItem('tracehub_token') || ''
  auth.setSession(t, {
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
    if (data.code !== 0) throw new Error(data.message || '加载失败')
    me.value = data.data
    bind.username = data.data.username || ''
    bind.webhook_url = data.data.webhook_url || ''
    ipText.value = (data.data.ip_whitelist || []).join('\n')
    syncAuthFromMe()
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : '加载失败'
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
    if (data.code !== 0) throw new Error(data.message || '保存失败')
    ok.value = '资料已保存'
    await load()
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : '保存失败'
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
    if (data.code !== 0) throw new Error(data.message || '保存失败')
    ok.value = 'IP 白名单已更新（空表示不限制账号级 IP）'
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : '保存失败'
  }
}

async function changePwd() {
  ok.value = ''
  err.value = ''
  if (pwd.new1 !== pwd.new2) {
    err.value = '两次新密码不一致'
    return
  }
  try {
    const { data } = await http.put<Envelope<null>>('/api/user/password', {
      old_password: pwd.old,
      new_password: pwd.new1,
    })
    if (data.code !== 0) throw new Error(data.message || '修改失败')
    ok.value = '密码已更新，请牢记新密码'
    pwd.old = ''
    pwd.new1 = ''
    pwd.new2 = ''
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : '修改失败'
  }
}

onMounted(load)
</script>

<template>
  <div class="max-w-xl space-y-10">
    <div>
      <h1 class="mb-2 text-2xl font-bold">账户设置</h1>
      <p class="text-sm text-slate-500">修改密码、昵称、Webhook 回调与账号级 IP 白名单。</p>
    </div>
    <p v-if="ok" class="text-sm text-emerald-400">{{ ok }}</p>
    <p v-if="err" class="text-sm text-red-400">{{ err }}</p>

    <section v-if="me" class="rounded-xl border border-slate-800 bg-slate-900/50 p-5">
      <h2 class="mb-3 text-sm font-semibold text-slate-300">账号信息</h2>
      <p class="text-sm"><span class="text-slate-500">邮箱</span> {{ me.email }}</p>
      <p class="mt-1 text-sm"><span class="text-slate-500">分组</span> {{ me.group_name || '—' }}</p>
    </section>

    <section class="rounded-xl border border-slate-800 bg-slate-900/50 p-5">
      <h2 class="mb-3 text-sm font-semibold text-slate-300">昵称与 Webhook</h2>
      <p class="mb-3 text-xs text-slate-500">Webhook 用于接收平台事件回调（与文档中的用量/余额回调配合，网关侧可按后续版本推送）。</p>
      <label class="mb-3 block text-sm">
        <span class="text-slate-500">显示名 / 昵称</span>
        <input v-model="bind.username" class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-3 py-2" />
      </label>
      <label class="mb-3 block text-sm">
        <span class="text-slate-500">回调 URL</span>
        <input v-model="bind.webhook_url" type="url" placeholder="https://..." class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 text-sm" />
      </label>
      <button type="button" class="rounded-lg bg-cyan-600 px-4 py-2 text-sm font-medium text-white hover:bg-cyan-500" @click="saveBind">保存</button>
    </section>

    <section class="rounded-xl border border-slate-800 bg-slate-900/50 p-5">
      <h2 class="mb-3 text-sm font-semibold text-slate-300">账号级 IP 白名单</h2>
      <p class="mb-3 text-xs text-slate-500">每行一个 IP 或 CIDR；留空表示不限制。API Key 级白名单可在 API Key 管理中单独配置。</p>
      <textarea v-model="ipText" rows="5" class="w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs" placeholder="180.200.0.0/16" />
      <button type="button" class="mt-3 rounded-lg border border-slate-600 px-4 py-2 text-sm hover:bg-slate-800" @click="saveIp">保存 IP 白名单</button>
    </section>

    <section class="rounded-xl border border-slate-800 bg-slate-900/50 p-5">
      <h2 class="mb-3 text-sm font-semibold text-slate-300">修改密码</h2>
      <label class="mb-2 block text-sm">
        <span class="text-slate-500">当前密码</span>
        <input v-model="pwd.old" type="password" class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-3 py-2" />
      </label>
      <label class="mb-2 block text-sm">
        <span class="text-slate-500">新密码</span>
        <input v-model="pwd.new1" type="password" class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-3 py-2" />
      </label>
      <label class="mb-3 block text-sm">
        <span class="text-slate-500">确认新密码</span>
        <input v-model="pwd.new2" type="password" class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-3 py-2" />
      </label>
      <button type="button" class="rounded-lg bg-slate-700 px-4 py-2 text-sm hover:bg-slate-600" @click="changePwd">更新密码</button>
    </section>
  </div>
</template>
