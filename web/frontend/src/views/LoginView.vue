<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter, useRoute, RouterLink } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import AuthFeatureAside from '../components/AuthFeatureAside.vue'

const { t, locale } = useI18n()

type LoginMode = 'password' | 'code'
const mode = ref<LoginMode>('password')

/** 密码模式：邮箱或用户名；验证码模式：注册邮箱 */
const account = ref('')
const password = ref('')
const code = ref('')
const err = ref('')
const loading = ref(false)
const sendLoading = ref(false)
const cooldown = ref(0)
let cdTimer: ReturnType<typeof setInterval> | null = null

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()

watch(mode, () => {
  err.value = ''
  code.value = ''
})

onUnmounted(() => {
  if (cdTimer) clearInterval(cdTimer)
})

const redirectLabel = computed(() => {
  const r = route.query.redirect
  const path = typeof r === 'string' ? r : Array.isArray(r) ? r[0] : ''
  if (!path || !path.startsWith('/') || path === '/console' || path === '/console/') return ''
  return path.length > 52 ? `${path.slice(0, 50)}…` : path
})

function startCooldown(sec: number) {
  if (cdTimer) clearInterval(cdTimer)
  cooldown.value = Math.max(0, Math.floor(sec))
  cdTimer = setInterval(() => {
    cooldown.value -= 1
    if (cooldown.value <= 0 && cdTimer) {
      clearInterval(cdTimer)
      cdTimer = null
    }
  }, 1000)
}

async function sendCode() {
  err.value = ''
  const em = account.value.trim()
  if (!em.includes('@')) {
    err.value = t('login.codeEmailInvalid')
    return
  }
  sendLoading.value = true
  try {
    const next = await auth.sendLoginCode(em, locale.value)
    startCooldown(typeof next === 'number' && next > 0 ? next : 60)
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('login.sendCodeFail')
  } finally {
    sendLoading.value = false
  }
}

async function submit() {
  err.value = ''
  loading.value = true
  try {
    if (mode.value === 'code') {
      await auth.loginWithEmailCode(account.value, code.value)
    } else {
      await auth.login(account.value, password.value)
    }
    const redir = (route.query.redirect as string) || '/console'
    router.push(redir)
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('login.fail')
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
          <div class="th-logo-mark th-logo-mark--lg th-logo-mark--image" role="img" aria-label="TraceHub">
            <img src="/tracehub-icon.svg" alt="" width="48" height="48" decoding="async" />
          </div>
        </div>
        <h1 class="mb-1 text-center text-2xl font-semibold tracking-tight text-slate-100">{{ t('login.title') }}</h1>
        <p class="mb-1 text-center text-sm text-slate-500">{{ t('login.subtitle') }}</p>
        <p class="mb-5 text-center text-[11px] leading-relaxed text-slate-600">
          {{ mode === 'code' ? t('login.hintCode') : t('login.hint') }}
        </p>
        <div class="th-console-muted-bar th-console-muted-bar--sm mb-6 flex justify-center gap-1 p-1">
          <button
            type="button"
            class="flex-1 rounded-md px-3 py-2 text-xs font-medium transition"
            :class="mode === 'password' ? 'bg-white/10 text-slate-100 shadow-sm' : 'text-slate-500 hover:text-slate-300'"
            @click="mode = 'password'"
          >
            {{ t('login.modePassword') }}
          </button>
          <button
            type="button"
            class="flex-1 rounded-md px-3 py-2 text-xs font-medium transition"
            :class="mode === 'code' ? 'bg-white/10 text-slate-100 shadow-sm' : 'text-slate-500 hover:text-slate-300'"
            @click="mode = 'code'"
          >
            {{ t('login.modeCode') }}
          </button>
        </div>
        <p
          v-if="redirectLabel"
          class="mb-5 rounded-lg border border-cyan-500/15 bg-cyan-950/25 px-3 py-2 text-center text-[11px] leading-relaxed text-cyan-200/85"
        >
          {{ t('login.redirectPrefix') }}
          <code class="th-dash-code-inline mx-0.5 px-1.5 py-0.5 text-[10px]">{{ redirectLabel }}</code>
        </p>
        <form class="space-y-5" @submit.prevent="submit">
          <div>
            <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
              mode === 'code' ? t('login.codeEmailLabel') : t('login.accountLabel')
            }}</label>
            <input
              v-model="account"
              :type="mode === 'code' ? 'email' : 'text'"
              required
              class="th-input"
              :autocomplete="mode === 'code' ? 'email' : 'username'"
              :placeholder="mode === 'code' ? t('login.codeEmailPlaceholder') : t('login.accountPlaceholder')"
            />
          </div>
          <div v-if="mode === 'password'">
            <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{ t('login.password') }}</label>
            <input v-model="password" type="password" required class="th-input" autocomplete="current-password" />
          </div>
          <div v-else class="flex flex-col gap-2 sm:flex-row sm:items-end">
            <div class="min-w-0 flex-1">
              <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{ t('login.codeLabel') }}</label>
              <input
                v-model="code"
                type="text"
                inputmode="numeric"
                maxlength="6"
                pattern="[0-9]*"
                required
                class="th-input font-mono tracking-widest"
                autocomplete="one-time-code"
                :placeholder="t('login.codePlaceholder')"
              />
            </div>
            <button
              type="button"
              :disabled="sendLoading || cooldown > 0"
              class="shrink-0 rounded-lg border border-cyan-500/30 bg-cyan-950/40 px-4 py-3 text-sm font-medium text-cyan-100/95 transition hover:border-cyan-400/40 hover:bg-cyan-900/35 disabled:cursor-not-allowed disabled:opacity-45"
              @click="sendCode"
            >
              {{
                sendLoading
                  ? t('common.sending')
                  : cooldown > 0
                    ? t('login.sendCodeWait', { n: cooldown })
                    : t('login.sendCode')
              }}
            </button>
          </div>
          <p v-if="err" class="rounded-lg border border-red-500/20 bg-red-950/40 px-3 py-2 text-sm text-red-300">{{ err }}</p>
          <button type="submit" :disabled="loading" class="th-btn-primary w-full py-3">
            {{ loading ? t('common.loggingIn') : t('login.submit') }}
          </button>
        </form>
        <p class="mt-8 text-center text-sm text-slate-500">
          {{ t('login.noAccount') }}
          <RouterLink to="/register" class="th-link-accent font-medium">{{ t('nav.register') }}</RouterLink>
        </p>
        <p class="mt-6 flex flex-wrap items-center justify-center gap-x-3 gap-y-1 text-center text-sm">
          <RouterLink to="/" class="text-slate-600 transition hover:text-slate-400">{{ t('nav.backHome') }}</RouterLink>
          <span class="text-slate-700" aria-hidden="true">·</span>
          <RouterLink to="/docs" class="text-slate-600 transition hover:text-slate-400">{{ t('nav.docsLong') }}</RouterLink>
        </p>
      </div>
      </div>
      <AuthFeatureAside class="lg:hidden" compact />
    </div>
  </div>
</template>
