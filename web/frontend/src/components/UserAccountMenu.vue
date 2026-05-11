<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const menu = ref<HTMLDetailsElement | null>(null)

function closeMenu() {
  if (menu.value) menu.value.open = false
}

const props = withDefaults(
  defineProps<{
    accent?: 'cyan' | 'amber'
  }>(),
  { accent: 'cyan' },
)

const auth = useAuthStore()
const router = useRouter()

const display = computed(() => auth.accountDisplay.trim() || '账号')

const subline = computed(() => {
  const u = auth.user
  if (!u?.email?.trim()) return ''
  if (display.value === u.email.trim()) return ''
  return u.email.trim()
})

const summaryRing = computed(() =>
  props.accent === 'amber'
    ? 'ring-1 ring-amber-500/15 hover:ring-amber-400/30 hover:bg-amber-950/30'
    : 'ring-1 ring-cyan-500/15 hover:ring-cyan-400/25 hover:bg-cyan-950/20',
)

const avatarClass = computed(() =>
  props.accent === 'amber'
    ? 'bg-gradient-to-br from-amber-300 to-orange-600 text-slate-950 shadow-md shadow-amber-500/20'
    : 'bg-gradient-to-br from-cyan-400 to-teal-600 text-slate-950 shadow-md shadow-cyan-500/25',
)

const dropdownClass = computed(() =>
  props.accent === 'amber'
    ? 'border-amber-500/15 bg-slate-950/95 shadow-2xl shadow-black/50'
    : 'border-cyan-500/10 bg-slate-950/95 shadow-2xl shadow-black/50',
)

function logout() {
  closeMenu()
  auth.logout()
  router.push('/')
}
</script>

<template>
  <details ref="menu" class="th-account-menu group relative">
    <summary
      class="flex cursor-pointer list-none items-center gap-2.5 rounded-xl py-1.5 pl-1.5 pr-2 outline-none transition"
      :class="summaryRing"
    >
      <span
        class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl text-sm font-bold"
        :class="avatarClass"
        :title="display"
      >
        {{ auth.avatarInitial }}
      </span>
      <div class="hidden min-w-0 flex-col text-left sm:flex">
        <span class="max-w-[168px] truncate text-sm font-semibold tracking-tight text-slate-100">{{ display }}</span>
        <span v-if="subline" class="max-w-[168px] truncate text-xs text-slate-500">{{ subline }}</span>
      </div>
      <svg class="h-4 w-4 shrink-0 text-slate-500 transition group-open:rotate-180" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
        <path fill-rule="evenodd" d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.94a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z" clip-rule="evenodd" />
      </svg>
    </summary>
    <div
      class="th-account-dropdown absolute right-0 z-50 mt-2 w-56 origin-top-right overflow-hidden rounded-xl border py-1.5 backdrop-blur-xl"
      :class="dropdownClass"
      @click.stop
    >
      <div class="border-b border-white/[0.06] px-4 py-3 sm:hidden">
        <p class="truncate text-sm font-semibold text-slate-100">{{ display }}</p>
        <p v-if="subline" class="truncate text-xs text-slate-500">{{ subline }}</p>
      </div>
      <RouterLink
        to="/console"
        class="block px-4 py-2.5 text-sm text-slate-300 transition hover:bg-white/[0.06] hover:text-white"
        @click="closeMenu"
      >
        用户控制台
      </RouterLink>
      <RouterLink
        to="/console/settings"
        class="block px-4 py-2.5 text-sm text-slate-400 transition hover:bg-white/[0.06] hover:text-slate-200"
        @click="closeMenu"
      >
        账户设置
      </RouterLink>
      <RouterLink
        to="/docs"
        class="block px-4 py-2.5 text-sm text-slate-400 transition hover:bg-white/[0.06] hover:text-slate-200"
        @click="closeMenu"
      >
        使用与 API 文档
      </RouterLink>
      <RouterLink
        v-if="auth.role === 'admin' || auth.role === 'superadmin'"
        to="/admin"
        class="block px-4 py-2.5 text-sm font-medium text-amber-400/95 transition hover:bg-amber-500/10 hover:text-amber-300"
        @click="closeMenu"
      >
        管理后台
      </RouterLink>
      <RouterLink to="/" class="block px-4 py-2.5 text-sm text-slate-500 transition hover:bg-white/[0.06] hover:text-slate-300" @click="closeMenu">
        返回首页
      </RouterLink>
      <button
        type="button"
        class="mt-1 w-full border-t border-white/[0.06] px-4 py-2.5 text-left text-sm text-rose-400/90 transition hover:bg-rose-500/10"
        @click="logout()"
      >
        退出登录
      </button>
    </div>
  </details>
</template>

<style scoped>
summary::-webkit-details-marker {
  display: none;
}
</style>
