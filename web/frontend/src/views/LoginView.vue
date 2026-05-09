<script setup lang="ts">
import { ref } from 'vue'
import { useRouter, useRoute, RouterLink } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const email = ref('')
const password = ref('')
const err = ref('')
const loading = ref(false)
const router = useRouter()
const route = useRoute()
const auth = useAuthStore()

async function submit() {
  err.value = ''
  loading.value = true
  try {
    await auth.login(email.value, password.value)
    const redir = (route.query.redirect as string) || '/console'
    router.push(redir)
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="th-page-shell flex min-h-screen items-center justify-center px-4 py-16">
    <div class="relative w-full max-w-[420px]">
      <div class="pointer-events-none absolute -inset-1 rounded-2xl bg-gradient-to-b from-cyan-500/20 via-transparent to-violet-600/10 opacity-80 blur-xl" />
      <div class="th-glass relative overflow-hidden p-8 md:p-10">
        <div class="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-cyan-400/50 to-transparent" />
        <div class="mb-8 flex justify-center">
          <div class="flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-400 to-teal-600 font-mono text-lg font-bold text-slate-950 shadow-lg shadow-cyan-500/30">TH</div>
        </div>
        <h1 class="mb-1 text-center text-2xl font-semibold tracking-tight text-white">欢迎回来</h1>
        <p class="mb-8 text-center text-sm text-slate-500">登录 TraceHub 控制台</p>
        <form class="space-y-5" @submit.prevent="submit">
          <div>
            <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">邮箱</label>
            <input v-model="email" type="email" required class="th-input" autocomplete="email" />
          </div>
          <div>
            <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">密码</label>
            <input v-model="password" type="password" required class="th-input" autocomplete="current-password" />
          </div>
          <p v-if="err" class="rounded-lg border border-red-500/20 bg-red-950/40 px-3 py-2 text-sm text-red-300">{{ err }}</p>
          <button type="submit" :disabled="loading" class="th-btn-primary w-full py-3">
            {{ loading ? '登录中…' : '登录' }}
          </button>
        </form>
        <p class="mt-8 text-center text-sm text-slate-500">
          没有账号？
          <RouterLink to="/register" class="font-medium text-cyan-400 transition hover:text-cyan-300">注册</RouterLink>
        </p>
        <p class="mt-6 text-center">
          <RouterLink to="/" class="text-sm text-slate-600 transition hover:text-slate-400">← 返回首页</RouterLink>
        </p>
      </div>
    </div>
  </div>
</template>
