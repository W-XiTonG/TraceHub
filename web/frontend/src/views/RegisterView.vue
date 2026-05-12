<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter, RouterLink } from 'vue-router'
import { useAuthStore, type RegisterEmailCodeMeta } from '../stores/auth'
import AuthFeatureAside from '../components/AuthFeatureAside.vue'

const { t, locale } = useI18n()

const email = ref('')
const password = ref('')
const username = ref('')
const code = ref('')
const err = ref('')
const ok = ref('')
const loading = ref(false)
const sendLoading = ref(false)
const meta = ref<RegisterEmailCodeMeta | null>(null)
const cooldown = ref(0)
let cdTimer: ReturnType<typeof setInterval> | null = null

const router = useRouter()
const auth = useAuthStore()

onMounted(async () => {
  try {
    meta.value = await auth.fetchRegisterEmailCodeMeta()
  } catch {
    meta.value = { required: false, can_send: false }
  }
})

onUnmounted(() => {
  if (cdTimer) clearInterval(cdTimer)
})

/** required 且不可发送时：按服务端 redis_ok / smtp_ready 给出明确原因 */
const emailCodeNotReadyHint = computed(() => {
  const m = meta.value
  if (!m?.required || m.can_send) return ''
  if (m.redis_ok === true && m.smtp_ready === false) return t('register.emailCodeNotReadySmtp')
  if (m.redis_ok === false && m.smtp_ready === true) return t('register.emailCodeNotReadyRedis')
  if (m.redis_ok === false && m.smtp_ready === false) return t('register.emailCodeNotReadyBoth')
  return t('register.emailCodeNotReady')
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
  const em = email.value.trim()
  if (!em.includes('@')) {
    err.value = t('register.codeEmailInvalid')
    return
  }
  sendLoading.value = true
  try {
    const next = await auth.sendRegisterCode(em, locale.value)
    startCooldown(typeof next === 'number' && next > 0 ? next : 60)
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('register.sendCodeFail')
  } finally {
    sendLoading.value = false
  }
}

async function submit() {
  err.value = ''
  ok.value = ''
  loading.value = true
  try {
    const needCode = meta.value?.required === true
    await auth.register(
      email.value,
      password.value,
      username.value || undefined,
      needCode ? code.value.trim() : undefined,
    )
    ok.value = t('register.ok')
    setTimeout(() => router.push('/login'), 1200)
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('register.fail')
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
        <h1 class="mb-1 text-center text-2xl font-semibold tracking-tight text-slate-100">{{ t('register.title') }}</h1>
        <p class="mb-1 text-center text-sm text-slate-500">{{ t('register.subtitle') }}</p>
        <p class="mb-5 text-center text-[11px] leading-relaxed text-slate-600">
          {{ meta?.required ? t('register.hintCode') : t('register.hint') }}
        </p>
        <p
          v-if="meta?.required && !meta.can_send"
          class="mb-5 rounded-lg border border-amber-500/25 bg-amber-950/30 px-3 py-2 text-center text-[11px] leading-relaxed text-amber-200/90"
        >
          {{ emailCodeNotReadyHint }}
        </p>
        <form class="space-y-5" @submit.prevent="submit">
          <div>
            <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{ t('register.email') }}</label>
            <input v-model="email" type="email" required class="th-input" autocomplete="email" />
          </div>
          <div>
            <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{ t('register.usernameOpt') }}</label>
            <input v-model="username" type="text" class="th-input" autocomplete="username" />
          </div>
          <div>
            <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{ t('register.password') }}</label>
            <input v-model="password" type="password" required minlength="6" class="th-input" autocomplete="new-password" />
          </div>
          <div v-if="meta?.required" class="flex flex-col gap-2 sm:flex-row sm:items-end">
            <div class="min-w-0 flex-1">
              <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{ t('register.codeLabel') }}</label>
              <input
                v-model="code"
                type="text"
                inputmode="numeric"
                maxlength="6"
                pattern="[0-9]*"
                :required="meta.required"
                class="th-input font-mono tracking-widest"
                autocomplete="one-time-code"
                :placeholder="t('register.codePlaceholder')"
              />
            </div>
            <button
              type="button"
              :disabled="sendLoading || cooldown > 0 || !meta.can_send"
              class="shrink-0 rounded-lg border border-cyan-500/30 bg-cyan-950/40 px-4 py-3 text-sm font-medium text-cyan-100/95 transition hover:border-cyan-400/40 hover:bg-cyan-900/35 disabled:cursor-not-allowed disabled:opacity-45"
              @click="sendCode"
            >
              {{
                sendLoading
                  ? t('common.sending')
                  : cooldown > 0
                    ? t('register.sendCodeWait', { n: cooldown })
                    : t('register.sendCode')
              }}
            </button>
          </div>
          <p v-if="err" class="rounded-lg border border-red-500/20 bg-red-950/40 px-3 py-2 text-sm text-red-300">{{ err }}</p>
          <p v-if="ok" class="rounded-lg border border-emerald-500/20 bg-emerald-950/40 px-3 py-2 text-sm text-emerald-300">{{ ok }}</p>
          <button type="submit" :disabled="loading" class="th-btn-primary w-full py-3">
            {{ loading ? t('common.submitting') : t('register.submit') }}
          </button>
        </form>
        <p class="mt-8 text-center text-sm text-slate-500">
          {{ t('register.hasAccount') }}
          <RouterLink to="/login" class="th-link-accent font-medium">{{ t('nav.login') }}</RouterLink>
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
