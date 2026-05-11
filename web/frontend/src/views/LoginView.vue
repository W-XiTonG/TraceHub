<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter, useRoute, RouterLink } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import AuthFeatureAside from '../components/AuthFeatureAside.vue'

/** 邮箱或用户名（与后端 JSON 字段 email 兼容） */
const account = ref('')
const password = ref('')
const err = ref('')
const loading = ref(false)
const router = useRouter()
const route = useRoute()
const auth = useAuthStore()

const redirectLabel = computed(() => {
  const r = route.query.redirect
  const path = typeof r === 'string' ? r : Array.isArray(r) ? r[0] : ''
  if (!path || !path.startsWith('/') || path === '/console' || path === '/console/') return ''
  return path.length > 52 ? `${path.slice(0, 50)}…` : path
})

async function submit() {
  err.value = ''
  loading.value = true
  try {
    await auth.login(account.value, password.value)
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
  <div class="th-page-shell th-auth-canvas flex min-h-screen flex-col justify-center px-4 py-12 lg:px-8 lg:py-16">
    <div class="relative z-[1] mx-auto flex w-full max-w-5xl flex-col gap-10 lg:flex-row lg:items-center lg:justify-between lg:gap-16">
      <AuthFeatureAside class="hidden lg:block lg:flex-1" />
      <div class="relative z-[1] w-full shrink-0 lg:w-[min(100%,26rem)]">
      <div class="th-glass th-auth-card-reveal relative overflow-hidden p-8 md:p-10">
        <div class="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-white/12 to-transparent" />
        <div class="mb-8 flex justify-center">
          <div class="th-logo-mark th-logo-mark--lg">TH</div>
        </div>
        <h1 class="mb-1 text-center text-2xl font-semibold tracking-tight text-slate-100">欢迎回来</h1>
        <p class="mb-1 text-center text-sm text-slate-500">登录 TraceHub 控制台</p>
        <p class="mb-8 text-center text-[11px] leading-relaxed text-slate-600">
          会话令牌由服务端签发；请在可信网络环境下访问，并妥善保管密码。
        </p>
        <p
          v-if="redirectLabel"
          class="mb-5 rounded-lg border border-cyan-500/15 bg-cyan-950/25 px-3 py-2 text-center text-[11px] leading-relaxed text-cyan-200/85"
        >
          登录成功后将前往
          <code class="mx-0.5 rounded bg-black/35 px-1.5 py-0.5 font-mono text-[10px] text-cyan-100/90">{{ redirectLabel }}</code>
        </p>
        <form class="space-y-5" @submit.prevent="submit">
          <div>
            <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">邮箱或用户名</label>
            <input v-model="account" type="text" required class="th-input" autocomplete="username" placeholder="例如 admin 或 your@email.com" />
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
          <RouterLink to="/register" class="th-link-accent font-medium">注册</RouterLink>
        </p>
        <p class="mt-6 text-center">
          <RouterLink to="/" class="text-sm text-slate-600 transition hover:text-slate-400">返回首页</RouterLink>
        </p>
      </div>
      </div>
      <AuthFeatureAside class="lg:hidden" compact />
    </div>
  </div>
</template>
