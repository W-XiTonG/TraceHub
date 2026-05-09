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
  <div class="flex min-h-screen items-center justify-center bg-slate-950 px-4">
    <div class="w-full max-w-md rounded-2xl border border-slate-800 bg-slate-900 p-8 shadow-xl">
      <h1 class="mb-6 text-center text-2xl font-bold text-white">注册</h1>
      <form class="space-y-4" @submit.prevent="submit">
        <div>
          <label class="mb-1 block text-sm text-slate-400">邮箱</label>
          <input v-model="email" type="email" required class="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-white outline-none focus:border-cyan-500" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-slate-400">用户名（可选）</label>
          <input v-model="username" type="text" class="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-white outline-none focus:border-cyan-500" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-slate-400">密码</label>
          <input v-model="password" type="password" required minlength="6" class="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-white outline-none focus:border-cyan-500" />
        </div>
        <p v-if="err" class="text-sm text-red-400">{{ err }}</p>
        <p v-if="ok" class="text-sm text-emerald-400">{{ ok }}</p>
        <button type="submit" :disabled="loading" class="w-full rounded-lg bg-cyan-500 py-2.5 font-semibold text-slate-950 hover:bg-cyan-400 disabled:opacity-50">
          {{ loading ? '提交中…' : '注册' }}
        </button>
      </form>
      <p class="mt-4 text-center text-sm text-slate-500">
        已有账号？
        <RouterLink to="/login" class="text-cyan-400 hover:underline">登录</RouterLink>
      </p>
    </div>
  </div>
</template>
