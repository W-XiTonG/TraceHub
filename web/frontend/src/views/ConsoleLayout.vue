<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'
import UserAccountMenu from '../components/UserAccountMenu.vue'
</script>

<template>
  <div class="flex min-h-screen bg-slate-950 text-slate-100">
    <aside class="w-56 shrink-0 border-r border-slate-800 bg-slate-900/80 p-4">
      <p class="mb-6 font-semibold text-cyan-400">用户控制台</p>
      <nav class="flex flex-col gap-2 text-sm">
        <RouterLink to="/console" class="rounded-lg px-3 py-2 hover:bg-slate-800" active-class="bg-slate-800 text-cyan-400">概览</RouterLink>
        <RouterLink to="/console/keys" class="rounded-lg px-3 py-2 hover:bg-slate-800" active-class="bg-slate-800 text-cyan-400">API Key</RouterLink>
        <RouterLink to="/console/logs" class="rounded-lg px-3 py-2 hover:bg-slate-800" active-class="bg-slate-800 text-cyan-400">调用日志</RouterLink>
        <RouterLink to="/console/group" class="rounded-lg px-3 py-2 hover:bg-slate-800" active-class="bg-slate-800 text-cyan-400">我的分组</RouterLink>
        <RouterLink to="/console/billing" class="rounded-lg px-3 py-2 hover:bg-slate-800" active-class="bg-slate-800 text-cyan-400">充值中心</RouterLink>
        <RouterLink to="/console/redeem" class="rounded-lg px-3 py-2 hover:bg-slate-800" active-class="bg-slate-800 text-cyan-400">兑换码</RouterLink>
        <RouterLink to="/console/settings" class="rounded-lg px-3 py-2 hover:bg-slate-800" active-class="bg-slate-800 text-cyan-400">账户设置</RouterLink>
      </nav>
    </aside>
    <div class="flex min-w-0 flex-1 flex-col">
      <header class="flex shrink-0 items-center justify-end border-b border-slate-800 bg-slate-950/90 px-4 py-2 backdrop-blur">
        <UserAccountMenu accent="cyan" />
      </header>
      <main class="flex-1 overflow-auto p-8">
        <RouterView />
      </main>
    </div>
  </div>
</template>
