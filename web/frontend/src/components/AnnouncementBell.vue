<script setup lang="ts">
import axios from 'axios'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import ThLoadingState from './ui/ThLoadingState.vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'
import { RouterLink } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { http, type Envelope } from '../api/http'

const props = withDefaults(
  defineProps<{
    accent: 'cyan' | 'amber'
    /** 查看全部公告（控制台概览锚点） */
    viewAllTo?: string
  }>(),
  { viewAllTo: '/console#dashboard-announcements' },
)

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
const { busy: loading, visible: loadingUi, run: runRefresh } = useSmoothLoading()
const err = ref('')
const latest = ref<Ann | null>(null)
const list = ref<Ann[]>([])
const viewingId = ref<number | null>(null)
/** localStorage 非响应式：写入已读后递增，让 hasUnread 重新计算 */
const seenRevision = ref(0)
const isCompact = ref(false)

const seenStorageKey = computed(() => {
  const id = auth.user?.id
  if (id != null && id > 0) return `${LS_PREFIX}${id}`
  const p = auth.token ? parseJwtUid(auth.token) : 0
  if (p > 0) return `${LS_PREFIX}${p}`
  return `${LS_PREFIX}0`
})

const displayAnn = computed(() => {
  if (viewingId.value != null) {
    const picked = list.value.find((a) => a.id === viewingId.value)
    if (picked) return picked
  }
  return latest.value
})

const otherAnnouncements = computed(() => {
  const cur = displayAnn.value
  if (!cur) return list.value
  return list.value.filter((a) => a.id !== cur.id)
})

const listItemActiveClass = computed(() =>
  props.accent === 'amber'
    ? 'bg-amber-500/10 ring-1 ring-amber-500/30'
    : 'bg-cyan-500/10 ring-1 ring-cyan-500/30',
)

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

const iconBtnClass = computed(() =>
  props.accent === 'amber' ? 'th-console-top-icon-btn th-console-top-icon-btn--amber' : 'th-console-top-icon-btn',
)

function syncCompact() {
  if (typeof window === 'undefined') return
  isCompact.value = window.matchMedia('(max-width: 1023px)').matches
}

async function refresh() {
  if (!auth.isLoggedIn) return
  err.value = ''
  await runRefresh(async () => {
    const { data } = await http.get<Envelope<{ announcement: Ann | null }>>('/api/user/announcements/latest')
    if (data.code !== 0) throw new Error(data.message || t('announcement.loadFail'))
    latest.value = data.data.announcement
    const { data: listData } = await http.get<Envelope<{ items: Ann[] }>>('/api/user/announcements', {
      params: { limit: 15 },
    })
    if (listData.code === 0) list.value = listData.data.items || []
    seenRevision.value++
    if (open.value && latest.value) {
      writeSeenId(latest.value.id)
    }
  }).catch((e: unknown) => {
    if (axios.isAxiosError(e) && e.response?.status === 401) return
    err.value = e instanceof Error ? e.message : t('announcement.loadFail')
  })
}

function toggle() {
  open.value = !open.value
}

function closePanel() {
  open.value = false
}

function selectAnnouncement(id: number) {
  viewingId.value = id
}

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

function onDocMouseDown(ev: MouseEvent) {
  if (!open.value || isCompact.value) return
  const el = rootRef.value
  if (el && !el.contains(ev.target as Node)) open.value = false
}

let pollTimer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  syncCompact()
  window.addEventListener('resize', syncCompact)
  document.addEventListener('mousedown', onDocMouseDown)
  void refresh()
  pollTimer = setInterval(() => void refresh(), 120_000)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', syncCompact)
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

watch(open, (isOpen) => {
  if (isOpen) {
    viewingId.value = null
    if (latest.value) writeSeenId(latest.value.id)
  }
})
</script>

<template>
  <div ref="rootRef" class="relative shrink-0">
    <button
      type="button"
      :class="iconBtnClass"
      :aria-label="t('announcement.aria')"
      :title="t('announcement.aria')"
      :aria-expanded="open"
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

    <!-- 桌面：下拉 -->
    <div
      v-if="open && !isCompact"
      class="absolute right-0 z-50 mt-2 flex w-[min(100vw-2rem,22rem)] max-h-[min(70vh,28rem)] flex-col overflow-hidden rounded-xl border border-white/[0.1] bg-slate-950/95 p-4 shadow-xl backdrop-blur-md"
      role="dialog"
      :aria-label="t('announcement.panelTitle')"
    >
      <div class="mb-3 flex shrink-0 items-start justify-between gap-2">
        <p class="text-sm font-semibold text-slate-100">{{ t('announcement.panelTitle') }}</p>
        <button type="button" class="rounded px-1.5 py-0.5 text-xs text-slate-500 hover:bg-white/10" @click="closePanel">
          {{ t('common.close') }}
        </button>
      </div>
      <div class="thin-scrollbar min-h-0 flex-1 overflow-y-auto">
        <ThLoadingState v-if="loadingUi" variant="inline" size="sm" />
        <p v-else-if="err" class="text-xs text-rose-300/90">{{ err }}</p>
        <template v-else-if="displayAnn">
          <p class="text-xs text-slate-500">{{ fmtTime(displayAnn.published_at) }}</p>
          <p class="mt-1 text-sm font-medium text-slate-100">{{ displayAnn.title }}</p>
          <p class="mt-2 whitespace-pre-wrap break-words text-xs leading-relaxed text-slate-300">{{ displayAnn.content }}</p>
        </template>
        <p v-else class="text-xs text-slate-500">{{ t('announcement.empty') }}</p>

        <div v-if="otherAnnouncements.length" class="mt-4 border-t border-white/[0.06] pt-3">
          <p class="mb-2 text-[11px] font-medium uppercase tracking-wide text-slate-500">{{ t('announcement.recent') }}</p>
          <ul class="space-y-1 text-xs">
            <li v-for="it in otherAnnouncements" :key="it.id">
              <button
                type="button"
                class="w-full rounded-lg px-2 py-1.5 text-left transition hover:bg-white/[0.06]"
                :class="displayAnn?.id === it.id ? listItemActiveClass : ''"
                @click="selectAnnouncement(it.id)"
              >
                <span class="font-medium text-slate-300">{{ it.title }}</span>
                <span class="ml-1 text-slate-600">· {{ fmtTime(it.published_at) }}</span>
              </button>
            </li>
          </ul>
        </div>
      </div>
      <div v-if="list.length" class="mt-3 shrink-0 border-t border-white/[0.06] pt-3">
        <RouterLink
          :to="viewAllTo"
          class="th-link-accent block text-center text-[11px] font-medium no-underline hover:underline"
          @click="closePanel"
        >
          {{ t('announcement.viewAllOnDashboard') }}
        </RouterLink>
      </div>
    </div>

    <!-- 小屏：底部抽屉 -->
    <Teleport to="body">
      <div v-if="open && isCompact" class="fixed inset-0 z-[200] flex flex-col justify-end" role="presentation">
        <button type="button" class="absolute inset-0 bg-black/55 backdrop-blur-[2px]" :aria-label="t('common.close')" @click="closePanel" />
        <div
          class="relative flex max-h-[min(85dvh,32rem)] w-full flex-col overflow-hidden rounded-t-2xl border border-white/[0.1] bg-slate-950/98 p-4 pb-[max(1rem,env(safe-area-inset-bottom))] shadow-2xl backdrop-blur-md"
          role="dialog"
          :aria-label="t('announcement.panelTitle')"
          @click.stop
        >
          <div class="mb-3 flex shrink-0 items-start justify-between gap-2">
            <p class="text-sm font-semibold text-slate-100">{{ t('announcement.panelTitle') }}</p>
            <button type="button" class="rounded px-1.5 py-0.5 text-xs text-slate-500 hover:bg-white/10" @click="closePanel">
              {{ t('common.close') }}
            </button>
          </div>
          <div class="thin-scrollbar min-h-0 flex-1 overflow-y-auto">
            <ThLoadingState v-if="loadingUi" variant="inline" size="sm" />
            <p v-else-if="err" class="text-xs text-rose-300/90">{{ err }}</p>
            <template v-else-if="displayAnn">
              <p class="text-xs text-slate-500">{{ fmtTime(displayAnn.published_at) }}</p>
              <p class="mt-1 text-sm font-medium text-slate-100">{{ displayAnn.title }}</p>
              <p class="mt-2 whitespace-pre-wrap break-words text-xs leading-relaxed text-slate-300">{{ displayAnn.content }}</p>
            </template>
            <p v-else class="text-xs text-slate-500">{{ t('announcement.empty') }}</p>

            <div v-if="otherAnnouncements.length" class="mt-4 border-t border-white/[0.06] pt-3">
              <p class="mb-2 text-[11px] font-medium uppercase tracking-wide text-slate-500">{{ t('announcement.recent') }}</p>
              <ul class="space-y-1 text-xs">
                <li v-for="it in otherAnnouncements" :key="it.id">
                  <button
                    type="button"
                    class="w-full rounded-lg px-2 py-1.5 text-left transition hover:bg-white/[0.06]"
                    :class="displayAnn?.id === it.id ? listItemActiveClass : ''"
                    @click="selectAnnouncement(it.id)"
                  >
                    <span class="font-medium text-slate-300">{{ it.title }}</span>
                    <span class="ml-1 break-words text-slate-600">· {{ fmtTime(it.published_at) }}</span>
                  </button>
                </li>
              </ul>
            </div>
          </div>
          <div v-if="list.length" class="mt-3 shrink-0 border-t border-white/[0.06] pt-3">
            <RouterLink
              :to="viewAllTo"
              class="th-link-accent block text-center text-[11px] font-medium no-underline hover:underline"
              @click="closePanel"
            >
              {{ t('announcement.viewAllOnDashboard') }}
            </RouterLink>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
