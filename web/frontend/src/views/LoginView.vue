<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'
import { useI18n } from 'vue-i18n'
import { useRouter, useRoute, RouterLink } from 'vue-router'
import axios from 'axios'
import { useAuthStore, apiErrorMessage, type LoginCaptchaResponse } from '../stores/auth'
import { useSiteMetaStore } from '../stores/siteMeta'
import AuthFeatureAside from '../components/AuthFeatureAside.vue'
import SiteBeianFooter from '../components/SiteBeianFooter.vue'
import ThPasswordInput from '../components/ui/ThPasswordInput.vue'
import { sanitizeAppRedirect } from '../utils/safeRedirect'

const { t, locale } = useI18n()
const siteMeta = useSiteMetaStore()

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

/** 密码登录表单内图形验证码 */
const captchaMeta = ref<LoginCaptchaResponse | null>(null)
const captchaAnswer = ref('')
const { busy: captchaLoading, visible: captchaLoadingUi, run: runCaptchaLoad } = useSmoothLoading({
  showDelay: 200,
  minVisible: 250,
})
const captchaErr = ref('')

/** 邮箱验证码「发送」前弹窗内图形验证码 */
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
const route = useRoute()
const auth = useAuthStore()

const captchaRequired = computed(() => captchaMeta.value?.enabled === true)

/** 主表单是否展示图形码（仅密码登录） */
const showFormCaptcha = computed(
  () => mode.value === 'password' && (captchaRequired.value || captchaLoading.value || !!captchaErr.value),
)

async function loadCaptcha() {
  captchaErr.value = ''
  await runCaptchaLoad(async () => {
    captchaMeta.value = await auth.fetchLoginCaptcha()
    captchaAnswer.value = ''
  }).catch((e: unknown) => {
    captchaMeta.value = null
    captchaErr.value = e instanceof Error ? e.message : t('login.captchaLoadFail')
  })
}

function captchaPayload():
  | { captcha_id: string; captcha_answer: string }
  | undefined {
  if (!captchaRequired.value || captchaMeta.value?.enabled !== true) return undefined
  return {
    captcha_id: captchaMeta.value.captcha_id,
    captcha_answer: captchaAnswer.value.trim(),
  }
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

/** 用户主动换图时清空错误提示 */
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

watch(mode, () => {
  err.value = ''
  code.value = ''
  sendCaptchaModalOpen.value = false
  modalErr.value = ''
  if (mode.value === 'password') {
    void loadCaptcha()
  }
})

onMounted(() => {
  void siteMeta.ensureLoaded()
  void loadCaptcha()
})

onUnmounted(() => {
  if (cdTimer) clearInterval(cdTimer)
})

const sessionExpired = computed(() => route.query.reason === 'session_expired')

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
  if (!captchaRequired.value) {
    try {
      await runSendLoginCode(em, undefined, 'main')
    } catch {
      /* err 由 runSendLoginCode 写入 */
    }
    return
  }
  sendCaptchaModalOpen.value = true
  modalErr.value = ''
  modalCaptchaAnswer.value = ''
  await loadModalCaptcha()
}

/** 发信接口中与「图形验证码」输入相关的错误：仅在小窗提示；其余错误关窗后在主表单提示 */
function isSendModalImageCaptchaError(e: unknown): boolean {
  const msg = apiErrorMessage(e, '')
  if (msg.includes('请填写图形验证码')) return true
  if (msg.includes('图形验证码错误或')) return true
  if (axios.isAxiosError(e) && e.response?.status === 401) return true
  return false
}

async function runSendLoginCode(
  email: string,
  captcha: { captcha_id: string; captcha_answer: string } | undefined,
  errTarget: 'main' | 'modal',
) {
  sendLoading.value = true
  try {
    const next = await auth.sendLoginCode(email, locale.value, captcha)
    startCooldown(typeof next === 'number' && next > 0 ? next : 60)
  } catch (e: unknown) {
    const msg = apiErrorMessage(e, t('login.sendCodeFail'))
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

async function confirmSendCaptchaModal() {
  modalErr.value = ''
  const em = account.value.trim()
  if (!modalCaptchaMeta.value?.enabled) {
    try {
      await runSendLoginCode(em, undefined, 'modal')
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
    await runSendLoginCode(em, cap, 'modal')
    closeSendCaptchaModal()
  } catch (e: unknown) {
    handleSendCodeModalError(e)
  }
}

/** 图形验证码类错误留在弹窗；其余错误关闭弹窗并在主表单提示 */
function handleSendCodeModalError(e: unknown) {
  if (isSendModalImageCaptchaError(e)) {
    if (!modalErr.value) {
      modalErr.value = apiErrorMessage(e, t('login.sendCodeFail'))
    }
    void loadModalCaptcha()
    return
  }
  err.value = apiErrorMessage(e, t('login.sendCodeFail'))
  closeSendCaptchaModal()
}

async function submit() {
  err.value = ''
  if (mode.value === 'password' && captchaRequired.value && !captchaAnswer.value.trim()) {
    err.value = t('login.captchaRequired')
    return
  }
  loading.value = true
  try {
    if (mode.value === 'code') {
      await auth.loginWithEmailCode(account.value, code.value)
    } else {
      await auth.login(account.value, password.value, captchaPayload())
    }
    const redir = sanitizeAppRedirect(route.query.redirect as string) || '/console'
    await router.push(redir)
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('login.fail')
    if (mode.value === 'password' && captchaRequired.value) await loadCaptcha()
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="th-page-shell th-auth-canvas flex min-h-screen flex-col px-4 py-12 lg:px-8 lg:py-16">
    <div class="relative z-[1] mx-auto flex w-full max-w-5xl flex-1 flex-col justify-center gap-10 lg:flex-row lg:items-center lg:justify-between lg:gap-16">
      <AuthFeatureAside class="hidden lg:block lg:flex-1" />
      <div class="relative z-[1] w-full shrink-0 lg:w-[min(100%,26rem)]">
      <div class="th-glass th-auth-card-reveal relative overflow-hidden p-8 md:p-10">
        <div class="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-white/12 to-transparent" />
        <div class="mb-8 flex justify-center">
          <div class="th-logo-mark th-logo-mark--lg th-logo-mark--image" role="img" :aria-label="siteMeta.displayName">
            <img :src="siteMeta.displayIconUrl" alt="" width="48" height="48" decoding="async" />
          </div>
        </div>
        <h1 class="mb-1 text-center text-2xl font-semibold tracking-tight text-slate-100">{{ t('login.title') }}</h1>
        <p class="mb-1 text-center text-sm text-slate-500">{{ t('login.subtitle', { brand: siteMeta.displayName }) }}</p>
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
          v-if="sessionExpired"
          class="mb-5 rounded-lg border border-amber-500/25 bg-amber-950/30 px-3 py-2 text-center text-xs leading-relaxed text-amber-100/90"
          role="status"
        >
          {{ t('login.sessionExpired') }}
        </p>
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
            <ThPasswordInput v-model="password" required autocomplete="current-password" />
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
              :disabled="sendLoading || cooldown > 0 || sendCaptchaModalOpen"
              class="th-btn-accent-soft shrink-0"
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

          <div
            v-if="showFormCaptcha"
            class="th-login-captcha-panel px-4 py-3.5 sm:px-4 sm:py-4"
          >
            <p v-if="captchaErr" class="text-center text-xs leading-relaxed text-amber-300/95">{{ captchaErr }}</p>
            <ThLoadingState v-else-if="captchaLoadingUi" variant="inline" size="sm" class="py-3" />
            <template v-else-if="captchaRequired && captchaMeta?.enabled">
              <p class="mb-3 text-center text-[10px] leading-relaxed text-slate-500 sm:text-left sm:text-[11px]">
                {{ t('login.captchaHint') }}
              </p>
              <div class="mb-4 flex justify-center">
                <button
                  type="button"
                  class="th-login-captcha-imgbtn"
                  :title="t('login.captchaRefresh')"
                  @click="loadCaptcha"
                >
                  <img
                    :src="captchaMeta.image_base64"
                    alt=""
                    class="block h-11 w-auto max-w-[min(100%,200px)] object-contain sm:h-12"
                    width="160"
                    height="60"
                    decoding="async"
                  />
                </button>
              </div>
              <div class="flex flex-col gap-2 sm:flex-row sm:items-end">
                <div class="min-w-0 flex-1">
                  <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
                    t('login.captchaLabel')
                  }}</label>
                  <input
                    v-model="captchaAnswer"
                    type="text"
                    inputmode="numeric"
                    autocomplete="one-time-code"
                    maxlength="8"
                    class="th-input w-full font-mono tracking-widest"
                    :placeholder="t('login.captchaPlaceholder')"
                  />
                </div>
                <button type="button" class="th-login-captcha-refresh" @click="loadCaptcha">
                  {{ t('login.captchaRefresh') }}
                </button>
              </div>
            </template>
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

    <Teleport to="body">
      <div
        v-if="sendCaptchaModalOpen"
        class="fixed inset-0 z-[200] flex items-center justify-center bg-black/65 p-4 backdrop-blur-[2px]"
        role="dialog"
        aria-modal="true"
        aria-labelledby="login-send-captcha-title"
        @click.self="closeSendCaptchaModal"
      >
        <div class="th-glass relative w-full max-w-[22rem] overflow-hidden rounded-xl p-5 shadow-2xl" @click.stop>
          <h2 id="login-send-captcha-title" class="mb-1 text-center text-sm font-semibold text-slate-100">
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
    <SiteBeianFooter class="relative z-[1] mt-auto shrink-0 pb-2" />
  </div>
</template>
