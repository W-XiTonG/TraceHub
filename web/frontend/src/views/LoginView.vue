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
  <div class="flex min-h-screen items-center justify-center bg-slate-950 px-4">
    <div class="w-full max-w-md rounded-2xl border border-slate-800 bg-slate-900 p-8 shadow-xl">
      <h1 class="mb-6 text-center text-2xl font-bold text-white">登录 TraceHub</h1>
      <form class="space-y-4" @submit.prevent="submit">
        <div>
          <label class="mb-1 block text-sm text-slate-400">邮箱</label>
          <input v-model="email" type="email" required class="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-white outline-none focus:border-cyan-500" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-slate-400">密码</label>
          <input v-model="password" type="password" required class="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-white outline-none focus:border-cyan-500" />
        </div>
        <p v-if="err" class="text-sm text-red-400">{{ err }}</p>
        <button type="submit" :disabled="loading" class="w-full rounded-lg bg-cyan-500 py-2.5 font-semibold text-slate-950 hover:bg-cyan-400 disabled:opacity-50">
          {{ loading ? '登录中…' : '登录' }}
        </button>
      </form>
      <p class="mt-4 text-center text-sm text-slate-500">
        没有账号？
        <RouterLink to="/register" class="text-cyan-400 hover:underline">注册</RouterLink>
      </p>
      <p class="mt-4 text-center">
        <RouterLink to="/" class="text-sm text-slate-500 hover:text-slate-300">← 返回首页</RouterLink>
      </p>
    </div>
  </div>
</template>
