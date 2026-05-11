<script setup lang="ts">
import { RouterLink, RouterView, useRoute } from 'vue-router'
import UserAccountMenu from '../components/UserAccountMenu.vue'

const route = useRoute()
</script>

<template>
  <div class="th-page-shell th-page-shell--admin flex h-[100dvh] min-h-0 overflow-hidden text-slate-100">
    <aside class="th-sidebar flex w-60 shrink-0 flex-col overflow-y-auto px-4 py-6">
      <RouterLink
        to="/"
        title="返回 TraceHub 官网首页"
        class="group mb-8 flex items-center gap-3 rounded-xl px-2 py-1.5 outline-none transition hover:bg-white/[0.05]"
      >
        <div class="th-logo-mark th-logo-mark--md th-logo-mark--amber shrink-0 transition group-hover:brightness-105">AD</div>
        <div class="min-w-0">
          <p class="th-sidebar-brand text-sm font-semibold text-amber-50/95">管理后台</p>
          <p class="text-[10px] uppercase tracking-[0.18em] text-slate-600">Admin</p>
        </div>
      </RouterLink>
      <nav class="flex flex-1 flex-col gap-0.5 text-sm">
        <RouterLink :to="{ name: 'admin-dash' }" class="th-nav-link th-nav-link--amber">概览</RouterLink>
        <RouterLink to="/admin/users" class="th-nav-link th-nav-link--amber">用户</RouterLink>
        <RouterLink to="/admin/channels" class="th-nav-link th-nav-link--amber">渠道</RouterLink>
        <RouterLink :to="{ name: 'admin-groups' }" class="th-nav-link th-nav-link--amber">分组</RouterLink>
        <RouterLink to="/admin/logs" class="th-nav-link th-nav-link--amber">调用日志</RouterLink>
        <RouterLink to="/admin/model-market" class="th-nav-link th-nav-link--amber">模型市场</RouterLink>
      </nav>
      <div class="mt-4 flex flex-col gap-2 border-t border-white/[0.06] pt-4">
        <RouterLink
          to="/console"
          title="切换到用户控制台（用量、API Key、充值等）"
          class="flex items-center gap-2 rounded-lg border border-cyan-500/25 bg-cyan-950/20 px-3 py-2 text-xs font-medium text-cyan-100/90 transition hover:border-cyan-400/40 hover:bg-cyan-950/35 hover:text-cyan-50"
        >
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-3.5 w-3.5 shrink-0 opacity-90" aria-hidden="true">
            <path d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4zM3 10a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H4a1 1 0 01-1-1v-6zM14 9a1 1 0 00-1 1v6a1 1 0 001 1h2a1 1 0 001-1v-6a1 1 0 00-1-1h-2z" />
          </svg>
          用户控制台
        </RouterLink>
        <RouterLink
          to="/"
          title="返回 TraceHub 官网首页"
          class="flex items-center gap-2 rounded-lg px-3 py-2 text-xs font-medium text-slate-500 transition hover:bg-white/[0.06] hover:text-slate-300"
        >
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-3.5 w-3.5 shrink-0 opacity-80" aria-hidden="true">
            <path d="M10.707 2.293a1 1 0 00-1.414 0l-7 7a1 1 0 101.414 1.414L4 10.414V17a1 1 0 001 1h2a1 1 0 001-1v-2a1 1 0 011-1h2a1 1 0 011 1v2a1 1 0 001 1h2a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7z" />
          </svg>
          官网首页
        </RouterLink>
      </div>
    </aside>
    <div class="flex min-h-0 min-w-0 flex-1 flex-col">
      <header class="th-topbar flex shrink-0 items-center justify-between gap-4 px-6 py-3.5">
        <div class="flex min-w-0 flex-1 flex-wrap items-center gap-x-4 gap-y-2">
          <div class="min-w-0">
            <p class="text-[11px] font-semibold uppercase tracking-[0.12em] text-amber-500/55">管理端</p>
            <p class="mt-0.5 text-[11px] text-slate-600">渠道、用户与模型目录</p>
          </div>
          <div class="flex shrink-0 flex-wrap items-center gap-2">
            <RouterLink
              to="/console"
              title="切换到用户控制台"
              class="inline-flex items-center gap-1.5 rounded-lg border border-cyan-500/25 bg-cyan-950/25 px-3 py-1.5 text-xs font-medium text-cyan-100/90 transition hover:border-cyan-400/40 hover:bg-cyan-950/40 hover:text-cyan-50"
            >
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-3.5 w-3.5 opacity-90" aria-hidden="true">
                <path d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4zM3 10a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H4a1 1 0 01-1-1v-6zM14 9a1 1 0 00-1 1v6a1 1 0 001 1h2a1 1 0 001-1v-6a1 1 0 00-1-1h-2z" />
              </svg>
              用户控制台
            </RouterLink>
            <RouterLink
              to="/"
              title="返回官网首页"
              class="inline-flex items-center gap-1.5 rounded-lg border border-amber-500/20 bg-amber-950/20 px-3 py-1.5 text-xs font-medium text-amber-200/90 transition hover:border-amber-400/35 hover:bg-amber-950/35 hover:text-amber-50"
            >
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-3.5 w-3.5 opacity-90" aria-hidden="true">
                <path d="M10.707 2.293a1 1 0 00-1.414 0l-7 7a1 1 0 101.414 1.414L4 10.414V17a1 1 0 001 1h2a1 1 0 001-1v-2a1 1 0 011-1h2a1 1 0 011 1v2a1 1 0 001 1h2a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7z" />
              </svg>
              官网首页
            </RouterLink>
          </div>
        </div>
        <UserAccountMenu accent="amber" />
      </header>
      <main class="min-h-0 flex-1 overflow-y-auto overflow-x-hidden p-6 md:px-10 md:pb-10 md:pt-8">
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
