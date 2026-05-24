<script setup lang="ts">
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { useSiteMetaStore } from '../stores/siteMeta'
import UserAccountMenu from '../components/UserAccountMenu.vue'
import AnnouncementBell from '../components/AnnouncementBell.vue'
import AdminGatewayAlertBell from '../components/AdminGatewayAlertBell.vue'
import AdminSidebarNav from '../components/shell/AdminSidebarNav.vue'
import ShellNavDrawer from '../components/shell/ShellNavDrawer.vue'
import SiteBeianFooter from '../components/SiteBeianFooter.vue'
import { useShellNavDrawer } from '../composables/useShellNavDrawer'

const route = useRoute()
const { t } = useI18n()
const siteMeta = useSiteMetaStore()
const { open: navOpen, close: closeNav, openDrawer } = useShellNavDrawer()

onMounted(() => void siteMeta.ensureLoaded())
</script>

<template>
  <div class="th-page-shell th-page-shell--admin flex h-[100dvh] min-h-0 overflow-hidden text-slate-100">
    <aside class="th-sidebar thin-scrollbar hidden w-60 shrink-0 flex-col overflow-y-auto px-4 py-6 lg:flex">
      <AdminSidebarNav />
    </aside>

    <ShellNavDrawer v-model="navOpen" accent="amber">
      <AdminSidebarNav @navigate="closeNav" />
    </ShellNavDrawer>

    <div class="flex min-h-0 min-w-0 flex-1 flex-col">
      <header class="th-topbar flex shrink-0 items-center justify-between gap-3 px-4 py-3 sm:gap-4 sm:px-6 sm:py-3.5">
        <div class="flex min-w-0 flex-1 items-center gap-2 sm:gap-3">
          <div class="shrink-0 lg:hidden">
            <button
              type="button"
              class="th-console-top-icon-btn"
              :aria-label="t('layout.openNav')"
              :aria-expanded="navOpen"
              @click="openDrawer"
            >
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-5 w-5" aria-hidden="true">
                <path
                  fill-rule="evenodd"
                  d="M2 4.75A.75.75 0 012.75 4h14.5a.75.75 0 010 1.5H2.75A.75.75 0 012 4.75zm0 5.5A.75.75 0 012.75 10h14.5a.75.75 0 010 1.5H2.75A.75.75 0 012 10.25zM2.75 15.5a.75.75 0 000 1.5h14.5a.75.75 0 000-1.5H2.75z"
                  clip-rule="evenodd"
                />
              </svg>
            </button>
          </div>
          <div class="flex min-w-0 flex-1 flex-wrap items-center gap-x-4 gap-y-2">
            <div class="min-w-0">
              <p class="text-[11px] font-semibold uppercase tracking-[0.12em] text-amber-500/55">{{ t('layout.adminTopSubtitle') }}</p>
              <p class="mt-0.5 hidden text-[11px] text-slate-600 sm:block">{{ t('layout.adminTopHint') }}</p>
            </div>
            <div class="hidden shrink-0 flex-wrap items-center gap-2 lg:flex">
              <RouterLink
                to="/console"
                :title="t('layout.adminConsoleTitle')"
                class="th-admin-user-console-link inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium"
              >
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-3.5 w-3.5 opacity-90" aria-hidden="true">
                  <path d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4zM3 10a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H4a1 1 0 01-1-1v-6zM14 9a1 1 0 00-1 1v6a1 1 0 001 1h2a1 1 0 001-1v-6a1 1 0 00-1-1h-2z" />
                </svg>
                {{ t('layout.adminConsoleShort') }}
              </RouterLink>
              <RouterLink
                to="/"
                :title="t('layout.adminBackTitle')"
                class="inline-flex items-center gap-1.5 rounded-lg border border-amber-500/20 bg-amber-950/20 px-3 py-1.5 text-xs font-medium text-amber-200/90 transition hover:border-amber-400/35 hover:bg-amber-950/35 hover:text-amber-50"
              >
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-3.5 w-3.5 opacity-90" aria-hidden="true">
                  <path d="M10.707 2.293a1 1 0 00-1.414 0l-7 7a1 1 0 101.414 1.414L4 10.414V17a1 1 0 001 1h2a1 1 0 001-1v-2a1 1 0 011-1h2a1 1 0 011 1v2a1 1 0 001 1h2a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7z" />
                </svg>
                {{ t('layout.adminHomeShort') }}
              </RouterLink>
            </div>
          </div>
        </div>
        <div class="flex shrink-0 items-center gap-2">
          <AdminGatewayAlertBell accent="amber" />
          <AnnouncementBell accent="amber" />
          <UserAccountMenu accent="amber" />
        </div>
      </header>
      <main class="thin-scrollbar min-h-0 flex-1 overflow-y-auto overflow-x-hidden p-4 sm:p-5 lg:p-6 md:px-10 md:pb-10 md:pt-8">
        <div class="th-main-inner">
          <RouterView v-slot="{ Component }">
            <Transition name="th-subpage" mode="out-in">
              <component :is="Component" :key="route.path" />
            </Transition>
          </RouterView>
        </div>
      </main>
      <SiteBeianFooter variant="shell" />
    </div>
  </div>
</template>
