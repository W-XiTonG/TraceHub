<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import UserAccountMenu from '../components/UserAccountMenu.vue'
import AnnouncementBell from '../components/AnnouncementBell.vue'

const route = useRoute()
const { t } = useI18n()
</script>

<template>
  <div class="th-page-shell th-page-shell--admin flex h-[100dvh] min-h-0 overflow-hidden text-slate-100">
    <aside class="th-sidebar thin-scrollbar flex w-60 shrink-0 flex-col overflow-y-auto px-4 py-6">
      <RouterLink
        to="/"
        :title="t('layout.adminBackTitle')"
        class="group mb-8 flex items-center gap-3 rounded-xl px-2 py-1.5 outline-none transition hover:bg-white/[0.05]"
      >
        <div class="th-logo-mark th-logo-mark--md th-logo-mark--amber shrink-0 transition group-hover:brightness-105">AD</div>
        <div class="min-w-0">
          <p class="th-sidebar-brand th-admin-sidebar-brand text-sm font-semibold">{{ t('layout.adminTitle') }}</p>
          <p class="text-[10px] uppercase tracking-[0.18em] text-slate-600">{{ t('layout.adminTag') }}</p>
        </div>
      </RouterLink>
      <nav class="flex flex-1 flex-col gap-0.5 text-sm">
        <RouterLink :to="{ name: 'admin-dash' }" class="th-nav-link th-nav-link--amber">{{ t('layout.adminNavOverview') }}</RouterLink>
        <RouterLink to="/admin/users" class="th-nav-link th-nav-link--amber">{{ t('layout.adminNavUsers') }}</RouterLink>
        <RouterLink to="/admin/channels" class="th-nav-link th-nav-link--amber">{{ t('layout.adminNavChannels') }}</RouterLink>
        <RouterLink :to="{ name: 'admin-groups' }" class="th-nav-link th-nav-link--amber">{{ t('layout.adminNavGroups') }}</RouterLink>
        <RouterLink to="/admin/logs" class="th-nav-link th-nav-link--amber">{{ t('layout.adminNavLogs') }}</RouterLink>
        <RouterLink to="/admin/model-market" class="th-nav-link th-nav-link--amber">{{ t('layout.adminNavMarket') }}</RouterLink>
        <RouterLink to="/admin/announcements" class="th-nav-link th-nav-link--amber">{{ t('layout.adminNavAnnouncements') }}</RouterLink>
        <RouterLink to="/admin/redeem-codes" class="th-nav-link th-nav-link--amber">{{ t('layout.adminNavRedeem') }}</RouterLink>
        <RouterLink :to="{ name: 'admin-system' }" class="th-nav-link th-nav-link--amber">{{ t('layout.adminNavSystem') }}</RouterLink>
        <RouterLink to="/docs" class="th-nav-link th-nav-link--amber border-t border-white/[0.06] pt-3 mt-2">{{ t('layout.navDocs') }}</RouterLink>
      </nav>
      <div class="mt-4 flex flex-col gap-2 border-t border-white/[0.06] pt-4">
        <RouterLink
          to="/console"
          :title="t('layout.adminConsoleTitle')"
          class="th-admin-user-console-link flex items-center gap-2 px-3 py-2 text-xs font-medium"
        >
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-3.5 w-3.5 shrink-0 opacity-90" aria-hidden="true">
            <path d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4zM3 10a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H4a1 1 0 01-1-1v-6zM14 9a1 1 0 00-1 1v6a1 1 0 001 1h2a1 1 0 001-1v-6a1 1 0 00-1-1h-2z" />
          </svg>
          {{ t('layout.adminConsoleShort') }}
        </RouterLink>
        <RouterLink
          to="/"
          :title="t('layout.adminBackTitle')"
          class="flex items-center gap-2 rounded-lg px-3 py-2 text-xs font-medium text-slate-500 transition hover:bg-white/[0.06] hover:text-slate-300"
        >
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-3.5 w-3.5 shrink-0 opacity-80" aria-hidden="true">
            <path d="M10.707 2.293a1 1 0 00-1.414 0l-7 7a1 1 0 101.414 1.414L4 10.414V17a1 1 0 001 1h2a1 1 0 001-1v-2a1 1 0 011-1h2a1 1 0 011 1v2a1 1 0 001 1h2a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7z" />
          </svg>
          {{ t('nav.siteHome') }}
        </RouterLink>
      </div>
    </aside>
    <div class="flex min-h-0 min-w-0 flex-1 flex-col">
      <header class="th-topbar flex shrink-0 items-center justify-between gap-4 px-6 py-3.5">
        <div class="flex min-w-0 flex-1 flex-wrap items-center gap-x-4 gap-y-2">
          <div class="min-w-0">
            <p class="text-[11px] font-semibold uppercase tracking-[0.12em] text-amber-500/55">{{ t('layout.adminTopSubtitle') }}</p>
            <p class="mt-0.5 text-[11px] text-slate-600">{{ t('layout.adminTopHint') }}</p>
          </div>
          <div class="flex shrink-0 flex-wrap items-center gap-2">
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
        <div class="flex shrink-0 items-center gap-2">
          <AnnouncementBell accent="amber" />
          <UserAccountMenu accent="amber" />
        </div>
      </header>
      <main class="thin-scrollbar min-h-0 flex-1 overflow-y-auto overflow-x-hidden p-6 md:px-10 md:pb-10 md:pt-8">
        <div class="th-main-inner">
          <RouterView v-slot="{ Component }">
            <Transition name="th-subpage" mode="out-in">
              <component :is="Component" :key="route.path" />
            </Transition>
          </RouterView>
        </div>
      </main>
    </div>
  </div>
</template>
