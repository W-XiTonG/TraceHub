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
    <aside class="w-56 shrink-0 border-r border-slate-800 bg-slate-900/80 p-4">
      <p class="mb-6 font-semibold text-cyan-400">用户控制台</p>
      <nav class="flex flex-col gap-2 text-sm">
        <RouterLink to="/console" class="rounded-lg px-3 py-2 hover:bg-slate-800" active-class="bg-slate-800 text-cyan-400">概览</RouterLink>
        <RouterLink to="/console/keys" class="rounded-lg px-3 py-2 hover:bg-slate-800" active-class="bg-slate-800 text-cyan-400">API Key</RouterLink>
        <RouterLink to="/console/redeem" class="rounded-lg px-3 py-2 hover:bg-slate-800" active-class="bg-slate-800 text-cyan-400">兑换码</RouterLink>
      </nav>
      <button type="button" class="mt-8 w-full rounded-lg border border-slate-700 py-2 text-sm text-slate-400 hover:bg-slate-800" @click="logout">退出</button>
    </aside>
    <main class="flex-1 overflow-auto p-8">
      <RouterView />
    </main>
  </div>
</template>
