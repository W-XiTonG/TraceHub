<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'
import UserAccountMenu from '../components/UserAccountMenu.vue'
</script>

<template>
  <div class="th-page-shell th-page-shell--admin flex h-[100dvh] min-h-0 overflow-hidden text-slate-100">
    <aside class="th-sidebar flex w-60 shrink-0 flex-col overflow-y-auto px-4 py-6">
      <div class="mb-8 flex items-center gap-3 px-2">
        <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-amber-300 to-orange-600 font-mono text-sm font-bold text-slate-950 shadow-lg shadow-amber-500/25">AD</div>
        <div>
          <p class="th-sidebar-brand text-sm font-semibold text-amber-100">管理后台</p>
          <p class="text-[10px] uppercase tracking-widest text-slate-600">Admin</p>
        </div>
      </div>
      <nav class="flex flex-1 flex-col gap-0.5 text-sm">
        <RouterLink :to="{ name: 'admin-dash' }" class="th-nav-link th-nav-link--amber">概览</RouterLink>
        <RouterLink to="/admin/users" class="th-nav-link th-nav-link--amber">用户</RouterLink>
        <RouterLink to="/admin/channels" class="th-nav-link th-nav-link--amber">渠道</RouterLink>
        <RouterLink to="/admin/logs" class="th-nav-link th-nav-link--amber">调用日志</RouterLink>
        <RouterLink to="/admin/model-market" class="th-nav-link th-nav-link--amber">模型市场</RouterLink>
      </nav>
      <RouterLink to="/console" class="mt-4 rounded-lg px-3 py-2 text-xs text-slate-600 transition hover:bg-white/5 hover:text-slate-400">← 用户控制台</RouterLink>
    </aside>
    <div class="flex min-h-0 min-w-0 flex-1 flex-col">
      <header class="th-topbar flex shrink-0 items-center justify-between px-6 py-3">
        <p class="text-xs font-medium uppercase tracking-widest text-amber-900/60">TraceHub Admin</p>
        <UserAccountMenu accent="amber" />
      </header>
      <main class="min-h-0 flex-1 overflow-y-auto overflow-x-hidden p-6 md:p-10">
        <RouterView />
      </main>
    </div>
  </div>
</template>
