<script setup lang="ts">
import { RouterLink, RouterView, useRoute } from 'vue-router'
import UserAccountMenu from '../components/UserAccountMenu.vue'

const route = useRoute()
</script>

<template>
  <div class="th-page-shell th-page-shell--admin flex h-[100dvh] min-h-0 overflow-hidden text-slate-100">
    <aside class="th-sidebar flex w-60 shrink-0 flex-col overflow-y-auto px-4 py-6">
      <div class="mb-8 flex items-center gap-3 px-2">
        <div class="th-logo-mark th-logo-mark--md th-logo-mark--amber shrink-0">AD</div>
        <div>
          <p class="th-sidebar-brand text-sm font-semibold text-amber-50/95">管理后台</p>
          <p class="text-[10px] uppercase tracking-[0.18em] text-slate-600">Admin</p>
        </div>
      </div>
      <nav class="flex flex-1 flex-col gap-0.5 text-sm">
        <RouterLink :to="{ name: 'admin-dash' }" class="th-nav-link th-nav-link--amber">概览</RouterLink>
        <RouterLink to="/admin/users" class="th-nav-link th-nav-link--amber">用户</RouterLink>
        <RouterLink to="/admin/channels" class="th-nav-link th-nav-link--amber">渠道</RouterLink>
        <RouterLink to="/admin/logs" class="th-nav-link th-nav-link--amber">调用日志</RouterLink>
        <RouterLink to="/admin/model-market" class="th-nav-link th-nav-link--amber">模型市场</RouterLink>
      </nav>
      <RouterLink to="/console" class="mt-4 rounded-lg px-3 py-2 text-xs text-slate-600 transition hover:bg-white/[0.04] hover:text-slate-400">用户控制台</RouterLink>
    </aside>
    <div class="flex min-h-0 min-w-0 flex-1 flex-col">
      <header class="th-topbar flex shrink-0 items-center justify-between px-6 py-3.5">
        <div>
          <p class="text-[11px] font-semibold uppercase tracking-[0.12em] text-amber-500/55">管理端</p>
          <p class="mt-0.5 text-[11px] text-slate-600">渠道、用户与模型目录</p>
        </div>
        <UserAccountMenu accent="amber" />
      </header>
      <main class="min-h-0 flex-1 overflow-y-auto overflow-x-hidden p-6 md:px-10 md:pb-10 md:pt-8">
        <div class="th-main-inner">
          <RouterView v-slot="{ Component }">
            <Transition name="th-page" mode="out-in">
              <component :is="Component" :key="route.path" />
            </Transition>
          </RouterView>
        </div>
      </main>
    </div>
  </div>
</template>
