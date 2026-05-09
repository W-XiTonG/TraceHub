<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useRouter } from 'vue-router'

const auth = useAuthStore()
const router = useRouter()

function logout() {
  auth.logout()
  router.push('/')
}
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
      </nav>
      <RouterLink to="/console" class="mt-6 block text-xs text-slate-500 hover:text-slate-300">← 用户控制台</RouterLink>
      <button type="button" class="mt-4 w-full rounded-lg border border-slate-700 py-2 text-sm text-slate-400 hover:bg-slate-800" @click="logout">退出</button>
    </aside>
    <main class="flex-1 overflow-auto p-8">
      <RouterView />
    </main>
  </div>
</template>
