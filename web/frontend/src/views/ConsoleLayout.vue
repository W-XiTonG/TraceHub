<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'
import UserAccountMenu from '../components/UserAccountMenu.vue'
</script>

<template>
  <div class="th-page-shell flex min-h-screen text-slate-100">
    <aside class="th-sidebar flex w-60 shrink-0 flex-col px-4 py-6">
      <div class="mb-8 flex items-center gap-3 px-2">
        <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-400 to-teal-600 font-mono text-sm font-bold text-slate-950 shadow-lg shadow-cyan-500/20">TH</div>
        <div>
          <p class="th-sidebar-brand text-sm font-semibold text-white">控制台</p>
          <p class="text-[10px] uppercase tracking-widest text-slate-600">User</p>
        </div>
      </div>
      <nav class="flex flex-1 flex-col gap-0.5 text-sm">
        <RouterLink :to="{ name: 'console' }" class="th-nav-link">概览</RouterLink>
        <RouterLink to="/console/keys" class="th-nav-link">API Key</RouterLink>
        <RouterLink to="/console/logs" class="th-nav-link">调用日志</RouterLink>
        <RouterLink to="/console/group" class="th-nav-link">我的分组</RouterLink>
        <RouterLink to="/console/billing" class="th-nav-link">充值中心</RouterLink>
        <RouterLink to="/console/redeem" class="th-nav-link">兑换码</RouterLink>
        <RouterLink to="/console/settings" class="th-nav-link">账户设置</RouterLink>
      </nav>
      <RouterLink to="/" class="mt-4 rounded-lg px-3 py-2 text-xs text-slate-600 transition hover:bg-white/5 hover:text-slate-400">← 返回官网</RouterLink>
    </aside>
    <div class="flex min-w-0 flex-1 flex-col">
      <header class="th-topbar flex shrink-0 items-center justify-between px-6 py-3">
        <p class="text-xs font-medium uppercase tracking-widest text-slate-600">TraceHub Console</p>
        <UserAccountMenu accent="cyan" />
      </header>
      <main class="flex-1 overflow-auto p-6 md:p-10">
        <RouterView />
      </main>
    </div>
  </div>
</template>
