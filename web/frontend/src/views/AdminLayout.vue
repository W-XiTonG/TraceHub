<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'
import UserAccountMenu from '../components/UserAccountMenu.vue'
</script>

<template>
  <div class="flex min-h-screen bg-slate-950 text-slate-100">
    <aside class="w-56 shrink-0 border-r border-slate-800 bg-slate-900/90 p-4">
      <p class="mb-6 font-semibold text-amber-400">管理后台</p>
      <nav class="flex flex-col gap-2 text-sm">
        <RouterLink to="/admin" class="rounded-lg px-3 py-2 hover:bg-slate-800" active-class="bg-slate-800 text-amber-400">概览</RouterLink>
        <RouterLink to="/admin/users" class="rounded-lg px-3 py-2 hover:bg-slate-800" active-class="bg-slate-800 text-amber-400">用户</RouterLink>
        <RouterLink to="/admin/channels" class="rounded-lg px-3 py-2 hover:bg-slate-800" active-class="bg-slate-800 text-amber-400">渠道</RouterLink>
        <RouterLink to="/admin/logs" class="rounded-lg px-3 py-2 hover:bg-slate-800" active-class="bg-slate-800 text-amber-400">调用日志</RouterLink>
        <RouterLink
          to="/admin/model-market"
          class="rounded-lg px-3 py-2 hover:bg-slate-800"
          active-class="bg-slate-800 text-amber-400"
        >
          模型市场
        </RouterLink>
      </nav>
      <RouterLink to="/console" class="mt-6 block text-xs text-slate-500 hover:text-slate-300">← 用户控制台</RouterLink>
    </aside>
    <div class="flex min-w-0 flex-1 flex-col">
      <header class="flex shrink-0 items-center justify-end border-b border-slate-800 bg-slate-950/90 px-4 py-2 backdrop-blur">
        <UserAccountMenu accent="amber" />
      </header>
      <main class="flex-1 overflow-auto p-8">
        <RouterView />
      </main>
    </div>
  </div>
</template>
