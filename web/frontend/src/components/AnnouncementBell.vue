<script setup lang="ts">
import axios from 'axios'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../stores/auth'
import { http, type Envelope } from '../api/http'

const props = defineProps<{
  accent: 'cyan' | 'amber'
}>()

const { t } = useI18n()
const auth = useAuthStore()

type Ann = {
  id: number
  title: string
  content: string
  published_at?: string
}

const LS_PREFIX = 'tracehub_announcement_seen_id:'

const rootRef = ref<HTMLElement | null>(null)
const open = ref(false)
const loading = ref(false)
const err = ref('')
const latest = ref<Ann | null>(null)
const list = ref<Ann[]>([])
/** localStorage 非响应式：写入已读后递增，让 hasUnread 重新计算 */
const seenRevision = ref(0)

const seenStorageKey = computed(() => {
  const id = auth.user?.id
  if (id != null && id > 0) return `${LS_PREFIX}${id}`
  const p = auth.token ? parseJwtUid(auth.token) : 0
  if (p > 0) return `${LS_PREFIX}${p}`
  return `${LS_PREFIX}0`
})

function parseJwtUid(access: string): number {
  try {
    const parts = access.split('.')
    if (parts.length < 2) return 0
    const payload = JSON.parse(atob(parts[1])) as { uid?: number }
    return typeof payload.uid === 'number' && !Number.isNaN(payload.uid) ? payload.uid : 0
  } catch {
    return 0
  }
}

function readSeenId(): number {
  try {
    const raw = localStorage.getItem(seenStorageKey.value)
    if (!raw) return 0
    const n = parseInt(raw, 10)
    return Number.isFinite(n) ? n : 0
  } catch {
    return 0
  }
}

function writeSeenId(id: number) {
  try {
    localStorage.setItem(seenStorageKey.value, String(id))
    seenRevision.value++
  } catch {
    /* ignore */
  }
}

const hasUnread = computed(() => {
  void seenRevision.value
  const a = latest.value
  if (!a) return false
  return a.id > readSeenId()
})

const ringClass =
  props.accent === 'amber'
    ? 'hover:border-amber-400/35 hover:bg-amber-950/30 hover:text-amber-100'
    : 'hover:border-cyan-400/35 hover:bg-cyan-950/30 hover:text-cyan-100'

async function refresh() {
  if (!auth.isLoggedIn) return
  loading.value = true
  err.value = ''
  try {
    const { data } = await http.get<Envelope<{ announcement: Ann | null }>>('/api/user/announcements/latest')
    if (data.code !== 0) throw new Error(data.message || t('announcement.loadFail'))
    latest.value = data.data.announcement
    const { data: listData } = await http.get<Envelope<{ items: Ann[] }>>('/api/user/announcements', {
      params: { limit: 15 },
    })
    if (listData.code === 0) list.value = listData.data.items || []
    // 轮询拉取后重新读 localStorage（例如其它标签页已标记已读）
    seenRevision.value++
    if (open.value && latest.value) {
      writeSeenId(latest.value.id)
    }
  } catch (e: unknown) {
    if (axios.isAxiosError(e) && e.response?.status === 401) return
    err.value = e instanceof Error ? e.message : t('announcement.loadFail')
  } finally {
    loading.value = false
  }
}

function toggle() {
  open.value = !open.value
  if (open.value && latest.value) writeSeenId(latest.value.id)
}

function onDocMouseDown(ev: MouseEvent) {
  if (!open.value) return
  const el = rootRef.value
  if (el && !el.contains(ev.target as Node)) open.value = false
}

let pollTimer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  document.addEventListener('mousedown', onDocMouseDown)
  void refresh()
  pollTimer = setInterval(() => void refresh(), 120_000)
})

onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDocMouseDown)
  if (pollTimer) clearInterval(pollTimer)
})

watch(
  () => auth.token,
  () => void refresh(),
)

watch(seenStorageKey, () => {
  seenRevision.value++
})

function fmtTime(iso?: string) {
  if (!iso) return ''
  try {
    const d = new Date(iso)
    if (Number.isNaN(d.getTime())) return iso
    return d.toLocaleString()
  } catch {
    return iso
  }
}
</script>

<template>
  <div ref="rootRef" class="relative shrink-0">
    <button
      type="button"
      class="relative inline-flex h-9 w-9 items-center justify-center rounded-lg border border-white/[0.1] bg-white/[0.04] text-slate-400 outline-none transition"
      :class="ringClass"
      :aria-label="t('announcement.aria')"
      :title="t('announcement.aria')"
      @click="toggle"
    >
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-5 w-5" aria-hidden="true">
        <path
          d="M10 2a6 6 0 00-6 6v3.586l-.707.707A1 1 0 004 14h12a1 1 0 00.707-1.707L16 11.586V8a6 6 0 00-6-6zM10 18a3 3 0 01-3-3h6a3 3 0 01-3 3z"
        />
      </svg>
      <span
        v-if="hasUnread"
        class="absolute right-1 top-1 h-2 w-2 rounded-full bg-rose-500 shadow-[0_0_0_2px_rgba(15,23,42,0.9)]"
        aria-hidden="true"
      />
    </button>

    <div
      v-if="open"
      class="absolute right-0 z-50 mt-2 w-[min(100vw-2rem,22rem)] rounded-xl border border-white/[0.1] bg-slate-950/95 p-4 shadow-xl backdrop-blur-md"
      role="dialog"
      :aria-label="t('announcement.panelTitle')"
    >
      <div class="mb-3 flex items-start justify-between gap-2">
        <p class="text-sm font-semibold text-slate-100">{{ t('announcement.panelTitle') }}</p>
        <button type="button" class="rounded px-1.5 py-0.5 text-xs text-slate-500 hover:bg-white/10" @click="open = false">
          {{ t('common.close') }}
        </button>
      </div>
      <p v-if="loading" class="text-xs text-slate-500">{{ t('common.loading') }}</p>
      <p v-else-if="err" class="text-xs text-rose-300/90">{{ err }}</p>
      <template v-else-if="latest">
        <p class="text-xs text-slate-500">{{ fmtTime(latest.published_at) }}</p>
        <p class="mt-1 text-sm font-medium text-slate-100">{{ latest.title }}</p>
        <p class="mt-2 max-h-40 overflow-y-auto whitespace-pre-wrap text-xs leading-relaxed text-slate-300">{{ latest.content }}</p>
      </template>
      <p v-else class="text-xs text-slate-500">{{ t('announcement.empty') }}</p>

      <div v-if="list.length > 1" class="mt-4 border-t border-white/[0.06] pt-3">
        <p class="mb-2 text-[11px] font-medium uppercase tracking-wide text-slate-500">{{ t('announcement.recent') }}</p>
        <ul class="max-h-36 space-y-2 overflow-y-auto text-xs">
          <li v-for="it in list.slice(1)" :key="it.id" class="text-slate-400">
            <span class="font-medium text-slate-300">{{ it.title }}</span>
            <span class="ml-1 text-slate-600">· {{ fmtTime(it.published_at) }}</span>
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>
