<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'
import { useI18n } from 'vue-i18n'
import { useRouter, RouterLink } from 'vue-router'
import axios from 'axios'
import { useAuthStore, apiErrorMessage, type RegisterEmailCodeMeta, type LoginCaptchaResponse } from '../stores/auth'
import { useSiteMetaStore } from '../stores/siteMeta'
import AuthFeatureAside from '../components/AuthFeatureAside.vue'
import RegisterPromoDropdown from '../components/RegisterPromoDropdown.vue'
import ThPasswordInput from '../components/ui/ThPasswordInput.vue'

const { t, locale } = useI18n()
const siteMeta = useSiteMetaStore()

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

/** 与登录邮箱发信一致：GET /api/user/login-captcha */
const captchaMeta = ref<LoginCaptchaResponse | null>(null)
const sendCaptchaModalOpen = ref(false)
const modalCaptchaMeta = ref<LoginCaptchaResponse | null>(null)
const modalCaptchaAnswer = ref('')
const {
  busy: modalCaptchaLoading,
  visible: modalCaptchaLoadingUi,
  run: runModalCaptchaLoad,
} = useSmoothLoading({ showDelay: 200, minVisible: 250 })
const modalErr = ref('')

const router = useRouter()
const auth = useAuthStore()

const captchaRequired = computed(() => captchaMeta.value?.enabled === true)

onMounted(async () => {
  void siteMeta.ensureLoaded()
  try {
    meta.value = await auth.fetchRegisterEmailCodeMeta()
  } catch {
    meta.value = { required: false, can_send: false }
  }
  try {
    captchaMeta.value = await auth.fetchLoginCaptcha()
  } catch {
    captchaMeta.value = { enabled: false }
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

async function loadModalCaptcha() {
  await runModalCaptchaLoad(async () => {
    modalCaptchaMeta.value = await auth.fetchLoginCaptcha()
    modalCaptchaAnswer.value = ''
  }).catch((e: unknown) => {
    modalCaptchaMeta.value = null
    modalErr.value = e instanceof Error ? e.message : t('login.captchaLoadFail')
  })
}

async function refreshModalCaptcha() {
  modalErr.value = ''
  await loadModalCaptcha()
}

function modalCaptchaPayload():
  | { captcha_id: string; captcha_answer: string }
  | undefined {
  if (!modalCaptchaMeta.value || modalCaptchaMeta.value.enabled !== true) return undefined
  return {
    captcha_id: modalCaptchaMeta.value.captcha_id,
    captcha_answer: modalCaptchaAnswer.value.trim(),
  }
}

function isSendModalImageCaptchaError(e: unknown): boolean {
  const msg = apiErrorMessage(e, '')
  if (msg.includes('请填写图形验证码')) return true
  if (msg.includes('图形验证码错误或')) return true
  if (axios.isAxiosError(e) && e.response?.status === 401) return true
  return false
}

async function runSendRegisterCode(
  em: string,
  captcha: { captcha_id: string; captcha_answer: string } | undefined,
  errTarget: 'main' | 'modal',
) {
  sendLoading.value = true
  try {
    const next = await auth.sendRegisterCode(em, locale.value, captcha)
    startCooldown(typeof next === 'number' && next > 0 ? next : 60)
  } catch (e: unknown) {
    const msg = apiErrorMessage(e, t('register.sendCodeFail'))
    if (errTarget === 'main') {
      err.value = msg
    } else if (isSendModalImageCaptchaError(e)) {
      modalErr.value = msg
    }
    throw e
  } finally {
    sendLoading.value = false
  }
}

function closeSendCaptchaModal() {
  sendCaptchaModalOpen.value = false
  modalErr.value = ''
  modalCaptchaAnswer.value = ''
}

function handleSendCodeModalError(e: unknown) {
  if (isSendModalImageCaptchaError(e)) {
    if (!modalErr.value) {
      modalErr.value = apiErrorMessage(e, t('register.sendCodeFail'))
    }
    void loadModalCaptcha()
    return
  }
  err.value = apiErrorMessage(e, t('register.sendCodeFail'))
  closeSendCaptchaModal()
}

async function confirmSendCaptchaModal() {
  modalErr.value = ''
  const em = email.value.trim()
  if (!modalCaptchaMeta.value?.enabled) {
    try {
      await runSendRegisterCode(em, undefined, 'modal')
      closeSendCaptchaModal()
    } catch (e: unknown) {
      handleSendCodeModalError(e)
    }
    return
  }
  if (!modalCaptchaAnswer.value.trim()) {
    modalErr.value = t('login.captchaRequired')
    return
  }
  const cap = modalCaptchaPayload()
  if (!cap) {
    modalErr.value = t('login.captchaLoadFail')
    return
  }
  try {
    await runSendRegisterCode(em, cap, 'modal')
    closeSendCaptchaModal()
  } catch (e: unknown) {
    handleSendCodeModalError(e)
  }
}

async function sendCode() {
  err.value = ''
  const em = email.value.trim()
  if (!em.includes('@')) {
    err.value = t('register.codeEmailInvalid')
    return
  }
  if (!captchaRequired.value) {
    try {
      await runSendRegisterCode(em, undefined, 'main')
    } catch {
      /* err 由 runSendRegisterCode 写入 */
    }
    return
  }
  sendCaptchaModalOpen.value = true
  modalErr.value = ''
  modalCaptchaAnswer.value = ''
  await loadModalCaptcha()
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
        <RegisterPromoDropdown variant="card" :text="siteMeta.registerPromoText" />
        <div class="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-white/12 to-transparent" />
        <div class="mb-6 flex justify-center">
          <div class="th-logo-mark th-logo-mark--lg th-logo-mark--image" role="img" :aria-label="siteMeta.displayName">
            <img :src="siteMeta.displayIconUrl" alt="" width="48" height="48" decoding="async" />
          </div>
        </div>
        <h1 class="mb-1 text-center text-2xl font-semibold tracking-tight text-slate-100">{{ t('register.title') }}</h1>
        <p class="mb-1 text-center text-sm text-slate-500">{{ t('register.subtitle', { brand: siteMeta.displayName }) }}</p>
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
            <ThPasswordInput v-model="password" required :minlength="6" autocomplete="new-password" />
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
              :disabled="sendLoading || cooldown > 0 || !meta.can_send || sendCaptchaModalOpen"
              class="th-btn-accent-soft shrink-0"
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

    <Teleport to="body">
      <div
        v-if="sendCaptchaModalOpen"
        class="fixed inset-0 z-[200] flex items-center justify-center bg-black/65 p-4 backdrop-blur-[2px]"
        role="dialog"
        aria-modal="true"
        aria-labelledby="register-send-captcha-title"
        @click.self="closeSendCaptchaModal"
      >
        <div class="th-glass relative w-full max-w-[22rem] overflow-hidden rounded-xl p-5 shadow-2xl" @click.stop>
          <h2 id="register-send-captcha-title" class="mb-1 text-center text-sm font-semibold text-slate-100">
            {{ t('login.sendCodeCaptchaTitle') }}
          </h2>
          <p class="mb-4 text-center text-[11px] leading-relaxed text-slate-500">
            {{ t('login.sendCodeCaptchaIntro') }}
          </p>

          <p v-if="modalErr" class="mb-3 rounded-lg border border-red-500/20 bg-red-950/35 px-3 py-2 text-xs text-red-300">
            {{ modalErr }}
          </p>
          <ThLoadingState v-if="modalCaptchaLoadingUi" class="mb-4 py-6" size="sm" />
          <template v-else-if="modalCaptchaMeta?.enabled">
            <div class="mb-4 flex justify-center">
              <button
                type="button"
                class="th-login-captcha-imgbtn"
                :title="t('login.captchaRefresh')"
                @click="refreshModalCaptcha"
              >
                <img
                  :src="modalCaptchaMeta.image_base64"
                  alt=""
                  class="block h-11 w-auto max-w-[min(100%,200px)] object-contain sm:h-12"
                  width="160"
                  height="60"
                  decoding="async"
                />
              </button>
            </div>
            <p class="mb-2 text-center text-[10px] text-slate-500">{{ t('login.captchaHint') }}</p>
            <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
              t('login.captchaLabel')
            }}</label>
            <input
              v-model="modalCaptchaAnswer"
              type="text"
              inputmode="numeric"
              autocomplete="one-time-code"
              maxlength="8"
              class="th-input mb-4 font-mono tracking-widest"
              :placeholder="t('login.captchaPlaceholder')"
              @keydown.enter.prevent="confirmSendCaptchaModal"
            />
            <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end sm:gap-2">
              <button
                type="button"
                class="th-login-modal-btn-ghost"
                :disabled="sendLoading"
                @click="closeSendCaptchaModal"
              >
                {{ t('common.cancel') }}
              </button>
              <button
                type="button"
                class="th-btn-accent-soft th-btn-accent-soft--compact"
                :disabled="sendLoading || modalCaptchaLoading"
                @click="confirmSendCaptchaModal"
              >
                {{ sendLoading ? t('common.sending') : t('login.sendCodeCaptchaConfirm') }}
              </button>
            </div>
          </template>
          <div v-else class="flex flex-col gap-2 sm:flex-row sm:justify-end">
            <button type="button" class="th-login-modal-btn-ghost" @click="closeSendCaptchaModal">
              {{ t('common.cancel') }}
            </button>
            <button type="button" class="th-btn-accent-soft th-btn-accent-soft--compact" @click="refreshModalCaptcha">
              {{ t('login.sendCodeCaptchaRetry') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
