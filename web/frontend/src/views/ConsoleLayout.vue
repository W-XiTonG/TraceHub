<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useSiteMetaStore } from '../stores/siteMeta'
import { http, type Envelope } from '../api/http'
import UserAccountMenu from '../components/UserAccountMenu.vue'
import AnnouncementBell from '../components/AnnouncementBell.vue'
import FeedbackModal from '../components/FeedbackModal.vue'

const { t } = useI18n()

const feedbackOpen = ref(false)

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const siteMeta = useSiteMetaStore()

const showAdminDenied = computed(() => route.query.denied === 'admin')

type MeLite = { balance?: number; balance_alert_threshold?: number }

const meForAlert = ref<MeLite | null>(null)

const showLowBalanceBanner = computed(() => {
  const m = meForAlert.value
  if (m == null || m.balance == null || m.balance_alert_threshold == null) return false
  const thr = Math.trunc(Number(m.balance_alert_threshold))
  if (thr <= 0) return false
  return Math.trunc(Number(m.balance)) <= thr
})

async function loadMeForBanner() {
  try {
    const { data } = await http.get<Envelope<MeLite>>('/api/user/me')
    if (data.code === 0 && data.data) meForAlert.value = data.data
  } catch {
    /* ignore */
  }
}

onMounted(() => {
  void siteMeta.ensureLoaded()
  void loadMeForBanner()
})

watch(
  () => route.path,
  (p) => {
    if (p === '/console' || p.startsWith('/console/')) void loadMeForBanner()
  },
)

function dismissAdminDenied() {
  const q = { ...route.query } as Record<string, string | string[]>
  delete q.denied
  router.replace({ path: route.path, query: q })
}

const canAccessAdmin = computed(() => ['admin', 'superadmin'].includes(auth.role))
</script>

<template>
  <div class="th-page-shell flex h-[100dvh] min-h-0 overflow-hidden text-slate-100">
    <aside class="th-sidebar thin-scrollbar flex w-60 shrink-0 flex-col overflow-y-auto px-4 py-6">
      <RouterLink
        to="/"
        :title="t('layout.consoleBackTitle')"
        class="group mb-8 flex items-center gap-3 rounded-xl px-2 py-1.5 outline-none transition hover:bg-white/[0.05]"
      >
        <div class="th-logo-mark th-logo-mark--md th-logo-mark--image shrink-0 transition group-hover:brightness-105">
          <img
            :key="siteMeta.displayIconUrl"
            :src="siteMeta.displayIconUrl"
            :alt="siteMeta.displayName"
            width="40"
            height="40"
            decoding="async"
          />
        </div>
        <div class="min-w-0">
          <p class="th-sidebar-brand text-sm font-semibold text-slate-100">
            <span>{{ siteMeta.displayName }}</span>
            <span class="font-normal text-slate-500"> · </span>
            <span class="text-slate-200">{{ t('layout.consoleTitle') }}</span>
          </p>
          <p class="text-[10px] uppercase tracking-[0.18em] text-slate-600">{{ t('layout.consoleUserTag') }}</p>
        </div>
      </RouterLink>
      <nav class="flex flex-1 flex-col gap-0.5 text-sm">
        <RouterLink :to="{ name: 'console' }" class="th-nav-link">{{ t('layout.navOverview') }}</RouterLink>
        <RouterLink :to="{ name: 'console-models' }" class="th-nav-link">{{ t('layout.navModels') }}</RouterLink>
        <RouterLink to="/console/keys" class="th-nav-link">{{ t('layout.navKeys') }}</RouterLink>
        <RouterLink to="/console/logs" class="th-nav-link">{{ t('layout.navLogs') }}</RouterLink>
        <RouterLink to="/console/balance-ledger" class="th-nav-link">{{ t('layout.navLedger') }}</RouterLink>
        <RouterLink to="/console/group" class="th-nav-link">{{ t('layout.navGroup') }}</RouterLink>
        <RouterLink to="/console/billing" class="th-nav-link">{{ t('layout.navBilling') }}</RouterLink>
        <RouterLink to="/console/redeem" class="th-nav-link">{{ t('layout.navRedeem') }}</RouterLink>
        <RouterLink to="/console/settings" class="th-nav-link">{{ t('layout.navSettings') }}</RouterLink>
        <RouterLink to="/docs" class="th-nav-link">{{ t('layout.navDocs') }}</RouterLink>
        <div class="mt-2 flex flex-col gap-0.5 border-t border-white/[0.06] pt-3">
          <RouterLink :to="{ name: 'console-playground' }" class="th-nav-link">{{ t('layout.navPlayground') }}</RouterLink>
          <RouterLink v-if="canAccessAdmin" to="/admin" class="th-nav-link">{{ t('layout.navAdmin') }}</RouterLink>
        </div>
      </nav>
      <RouterLink
        to="/"
        class="mt-4 flex items-center gap-2 rounded-lg px-3 py-2 text-xs font-medium text-slate-500 transition hover:bg-white/[0.06] hover:text-slate-300"
      >
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-3.5 w-3.5 shrink-0 opacity-80" aria-hidden="true">
          <path d="M10.707 2.293a1 1 0 00-1.414 0l-7 7a1 1 0 101.414 1.414L4 10.414V17a1 1 0 001 1h2a1 1 0 001-1v-2a1 1 0 011-1h2a1 1 0 011 1v2a1 1 0 001 1h2a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7z" />
        </svg>
        {{ t('nav.siteHome') }}
      </RouterLink>
    </aside>
    <div class="flex min-h-0 min-w-0 flex-1 flex-col">
      <header class="th-topbar flex shrink-0 items-center justify-between gap-4 px-6 py-3.5">
        <div class="flex min-w-0 flex-1 flex-wrap items-center gap-x-4 gap-y-2">
          <div class="min-w-0">
            <p class="th-topbar-title">{{ t('layout.consoleTitle') }}</p>
            <p class="mt-0.5 text-[11px] text-slate-600">{{ t('layout.consoleSubtitle') }}</p>
          </div>
          <div class="flex shrink-0 flex-wrap items-center gap-2">
            <RouterLink
              to="/docs"
              class="inline-flex items-center gap-1.5 rounded-lg border border-white/[0.1] bg-white/[0.04] px-3 py-1.5 text-xs font-medium text-slate-400 transition hover:border-cyan-500/25 hover:bg-cyan-950/25 hover:text-cyan-200/90"
            >
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-3.5 w-3.5 opacity-90" aria-hidden="true">
                <path d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4z" />
              </svg>
              {{ t('nav.docs') }}
            </RouterLink>
            <RouterLink
              to="/"
              class="inline-flex items-center gap-1.5 rounded-lg border border-white/[0.1] bg-white/[0.04] px-3 py-1.5 text-xs font-medium text-slate-400 transition hover:border-white/[0.14] hover:bg-white/[0.07] hover:text-slate-200"
            >
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-3.5 w-3.5 opacity-90" aria-hidden="true">
                <path d="M10.707 2.293a1 1 0 00-1.414 0l-7 7a1 1 0 101.414 1.414L4 10.414V17a1 1 0 001 1h2a1 1 0 001-1v-2a1 1 0 011-1h2a1 1 0 011 1v2a1 1 0 001 1h2a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7z" />
              </svg>
              {{ t('nav.siteHome') }}
            </RouterLink>
          </div>
        </div>
        <div class="flex shrink-0 items-center gap-2">
          <button
            type="button"
            class="th-console-top-icon-btn"
            :aria-label="t('layout.navFeedback')"
            :title="t('layout.navFeedback')"
            @click="feedbackOpen = true"
          >
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-5 w-5" aria-hidden="true">
              <path
                fill-rule="evenodd"
                d="M18 10c0 3.866-3.582 7-8 7a8.841 8.841 0 01-4.083-.98L2 17l1.338-3.123C2.493 12.767 2 11.434 2 10c0-3.866 3.582-7 8-7s8 3.134 8 7zM7 9H5v2h2V9zm8 0h-2v2h2V9zM9 9h2v2H9V9z"
                clip-rule="evenodd"
              />
            </svg>
          </button>
          <AnnouncementBell accent="cyan" />
          <UserAccountMenu accent="cyan" />
        </div>
      </header>
      <FeedbackModal v-model="feedbackOpen" />
      <main class="thin-scrollbar flex min-h-0 flex-1 flex-col overflow-y-auto overflow-x-hidden p-6 md:px-10 md:pb-10 md:pt-8">
        <Transition name="th-fade">
          <div
            v-if="showAdminDenied"
            class="mb-6 flex shrink-0 items-start justify-between gap-4 rounded-xl border border-amber-500/25 bg-amber-950/35 px-4 py-3 text-sm text-amber-100/95"
            role="alert"
          >
            <p>
              {{
                t('layout.adminDenied', {
                  roles: 'admin / superadmin',
                  roleCode: 'role',
                  bootstrapCode: 'bootstrap.admin_email',
                })
              }}
            </p>
            <button type="button" class="shrink-0 rounded-lg px-2 py-1 text-xs text-amber-200/80 hover:bg-white/10" @click="dismissAdminDenied">{{ t('common.close') }}</button>
          </div>
        </Transition>
        <Transition name="th-fade">
          <div
            v-if="showLowBalanceBanner"
            class="mb-6 flex shrink-0 flex-wrap items-start justify-between gap-4 rounded-xl border border-rose-500/30 bg-rose-950/40 px-4 py-3 text-sm text-rose-100/95"
            role="status"
          >
            <p class="min-w-0 flex-1">{{ t('layout.lowBalanceBanner') }}</p>
            <RouterLink
              to="/console/settings"
              class="shrink-0 rounded-lg border border-rose-400/35 bg-rose-900/40 px-3 py-1.5 text-xs font-medium text-rose-100/95 hover:bg-rose-900/55"
            >
              {{ t('layout.lowBalanceSettings') }}
            </RouterLink>
          </div>
        </Transition>
        <div class="th-main-inner flex min-h-0 min-w-0 flex-1 flex-col">
          <!-- 仅右侧主内容区过渡；壳层 key 在 App.vue 按 shell 固定，避免侧栏随路由 path 重挂载 -->
          <RouterView v-slot="{ Component }">
            <Transition name="th-subpage" mode="out-in">
              <component :is="Component" :key="route.path" class="th-route-view flex min-h-0 w-full flex-1 flex-col" />
            </Transition>
          </RouterView>
        </div>
      </main>
    </div>
  </div>
</template>
