<script setup lang="ts">
import { ref } from 'vue'
import { useRouter, RouterLink } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import AuthFeatureAside from '../components/AuthFeatureAside.vue'

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
  <div class="th-page-shell th-auth-canvas flex min-h-screen flex-col justify-center px-4 py-12 lg:px-8 lg:py-16">
    <div class="relative z-[1] mx-auto flex w-full max-w-5xl flex-col gap-10 lg:flex-row lg:items-center lg:justify-between lg:gap-16">
      <AuthFeatureAside class="hidden lg:block lg:flex-1" />
      <div class="w-full shrink-0 lg:w-[min(100%,26rem)]">
      <div class="th-glass th-auth-card-reveal relative overflow-hidden p-8 md:p-10">
        <div class="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-white/12 to-transparent" />
        <div class="mb-6 flex justify-center">
          <div class="th-logo-mark th-logo-mark--lg">TH</div>
        </div>
        <h1 class="mb-1 text-center text-2xl font-semibold tracking-tight text-slate-100">创建账号</h1>
        <p class="mb-1 text-center text-sm text-slate-500">开通控制台与 API Key</p>
        <p class="mb-8 text-center text-[11px] leading-relaxed text-slate-600">
          注册即表示您理解平台计费与安全策略；管理员可后续为您分配分组与额度。
        </p>
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
          <RouterLink to="/login" class="th-link-accent font-medium">登录</RouterLink>
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
