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
    /** cyan: 官网/控制台；amber: 管理后台 */
    accent?: 'cyan' | 'amber'
  }>(),
  { accent: 'cyan' },
)

const auth = useAuthStore()
const router = useRouter()

/** 主标题：与 store 一致；极端情况下无 JWT 信息时用「账号」占位，不再用「已登录」避免头像显示「已」 */
const display = computed(() => auth.accountDisplay.trim() || '账号')

const subline = computed(() => {
  const u = auth.user
  if (!u?.email?.trim()) return ''
  if (display.value === u.email.trim()) return ''
  return u.email.trim()
})

const ringClass = computed(() =>
  props.accent === 'amber'
    ? 'ring-amber-500/40 hover:ring-amber-400/60'
    : 'ring-cyan-500/40 hover:ring-cyan-400/60',
)

const avatarBg = computed(() =>
  props.accent === 'amber' ? 'bg-amber-600 text-slate-950' : 'bg-cyan-600 text-slate-950',
)

function logout() {
  closeMenu()
  auth.logout()
  router.push('/')
}
</script>

<template>
  <details ref="menu" class="group relative">
    <summary
      class="flex cursor-pointer list-none items-center gap-2 rounded-lg py-1 pl-1 pr-2 outline-none ring-1 ring-transparent transition hover:bg-slate-800/80"
      :class="ringClass"
    >
      <span
        class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-sm font-bold shadow-inner"
        :class="avatarBg"
        :title="display"
      >
        {{ auth.avatarInitial }}
      </span>
      <div class="hidden min-w-0 flex-col text-left sm:flex">
        <span class="max-w-[160px] truncate text-sm font-medium text-slate-100">{{ display }}</span>
        <span v-if="subline" class="max-w-[160px] truncate text-xs text-slate-500">{{ subline }}</span>
      </div>
      <svg class="h-4 w-4 shrink-0 text-slate-500 group-open:rotate-180 transition" viewBox="0 0 20 20" fill="currentColor">
        <path fill-rule="evenodd" d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.94a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z" clip-rule="evenodd" />
      </svg>
    </summary>
    <div
      class="absolute right-0 z-30 mt-1 w-52 origin-top-right rounded-xl border border-slate-700 bg-slate-900 py-1 shadow-xl"
      @click.stop
    >
      <div class="border-b border-slate-800 px-3 py-2 sm:hidden">
        <p class="truncate text-sm font-medium text-slate-100">{{ display }}</p>
        <p v-if="subline" class="truncate text-xs text-slate-500">{{ subline }}</p>
      </div>
      <RouterLink
        to="/console"
        class="block px-3 py-2 text-sm text-slate-300 hover:bg-slate-800 hover:text-white"
        @click="closeMenu"
      >
        用户控制台
      </RouterLink>
      <RouterLink
        v-if="auth.role === 'admin' || auth.role === 'superadmin'"
        to="/admin"
        class="block px-3 py-2 text-sm text-amber-400/90 hover:bg-slate-800 hover:text-amber-300"
        @click="closeMenu"
      >
        管理后台
      </RouterLink>
      <RouterLink to="/" class="block px-3 py-2 text-sm text-slate-400 hover:bg-slate-800 hover:text-slate-200" @click="closeMenu">
        返回首页
      </RouterLink>
      <button
        type="button"
        class="mt-1 w-full border-t border-slate-800 px-3 py-2 text-left text-sm text-red-400/90 hover:bg-slate-800"
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
