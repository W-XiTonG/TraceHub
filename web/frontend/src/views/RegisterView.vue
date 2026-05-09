<script setup lang="ts">
import { ref } from 'vue'
import { useRouter, RouterLink } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const email = ref('')
const password = ref('')
const username = ref('')
const err = ref('')
const ok = ref('')
const loading = ref(false)
const router = useRouter()
const auth = useAuthStore()

async function submit() {
  err.value = ''
  ok.value = ''
  loading.value = true
  try {
    await auth.register(email.value, password.value, username.value || undefined)
    ok.value = '注册成功，请登录'
    setTimeout(() => router.push('/login'), 1200)
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : '注册失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="th-page-shell flex min-h-screen items-center justify-center px-4 py-16">
    <div class="relative w-full max-w-[420px]">
      <div class="pointer-events-none absolute -inset-1 rounded-2xl bg-gradient-to-b from-teal-500/15 via-transparent to-cyan-600/10 opacity-80 blur-xl" />
      <div class="th-glass relative overflow-hidden p-8 md:p-10">
        <div class="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-teal-400/45 to-transparent" />
        <h1 class="mb-1 text-center text-2xl font-semibold tracking-tight text-white">创建账号</h1>
        <p class="mb-8 text-center text-sm text-slate-500">加入 TraceHub，开通 API 与控制台</p>
        <form class="space-y-5" @submit.prevent="submit">
          <div>
            <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">邮箱</label>
            <input v-model="email" type="email" required class="th-input" autocomplete="email" />
          </div>
          <div>
            <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">用户名（可选）</label>
            <input v-model="username" type="text" class="th-input" autocomplete="username" />
          </div>
          <div>
            <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">密码</label>
            <input v-model="password" type="password" required minlength="6" class="th-input" autocomplete="new-password" />
          </div>
          <p v-if="err" class="rounded-lg border border-red-500/20 bg-red-950/40 px-3 py-2 text-sm text-red-300">{{ err }}</p>
          <p v-if="ok" class="rounded-lg border border-emerald-500/20 bg-emerald-950/40 px-3 py-2 text-sm text-emerald-300">{{ ok }}</p>
          <button type="submit" :disabled="loading" class="th-btn-primary w-full py-3">
            {{ loading ? '提交中…' : '注册' }}
          </button>
        </form>
        <p class="mt-8 text-center text-sm text-slate-500">
          已有账号？
          <RouterLink to="/login" class="font-medium text-cyan-400 transition hover:text-cyan-300">登录</RouterLink>
        </p>
      </div>
    </div>
  </div>
</template>
