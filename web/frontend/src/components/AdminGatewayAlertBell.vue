<script setup lang="ts">
import axios from 'axios'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import ThLoadingState from './ui/ThLoadingState.vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'
import { useAuthStore } from '../stores/auth'
import { http, type Envelope } from '../api/http'

const props = withDefaults(
  defineProps<{
    accent?: 'cyan' | 'amber'
  }>(),
  { accent: 'amber' },
)

const { t } = useI18n()
const auth = useAuthStore()

type GwAlert = {
  id: number
  event_kind: string
  severity: string
  title: string
  summary?: string
  request_id?: string
  user_id?: number
  status_code?: number
  model?: string
  created_at: string
  unread?: boolean
}

const rootRef = ref<HTMLElement | null>(null)
const open = ref(false)
const { busy: loading, visible: loadingUi, run: runRefresh } = useSmoothLoading()
const err = ref('')
const items = ref<GwAlert[]>([])
const unreadCount = ref(0)
const viewingId = ref<number | null>(null)
const isCompact = ref(false)

const displayItem = computed(() => {
  if (viewingId.value != null) {
    return items.value.find((a) => a.id === viewingId.value) ?? items.value[0] ?? null
  }
  return items.value[0] ?? null
})

const otherItems = computed(() => {
  const cur = displayItem.value
  if (!cur) return items.value
  return items.value.filter((a) => a.id !== cur.id)
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
    const { data } = await http.get<Envelope<{ items: GwAlert[]; unread_count: number }>>('/api/admin/gateway-alerts', {
      params: { limit: 20 },
    })
    if (data.code !== 0) throw new Error(data.message || t('adminGatewayAlert.loadFail'))
    items.value = data.data?.items ?? []
    unreadCount.value = data.data?.unread_count ?? 0
  }).catch((e: unknown) => {
    if (axios.isAxiosError(e) && e.response?.status === 401) return
    err.value = e instanceof Error ? e.message : t('adminGatewayAlert.loadFail')
  })
}

async function markRead() {
  try {
    await http.post<Envelope<unknown>>('/api/admin/gateway-alerts/mark-read')
    unreadCount.value = 0
    items.value = items.value.map((x) => ({ ...x, unread: false }))
  } catch {
    /* ignore */
  }
}

function toggle() {
  open.value = !open.value
}

function closePanel() {
  open.value = false
}

function selectItem(id: number) {
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

function kindLabel(kind: string) {
  const key = `adminGatewayAlert.kind.${kind}` as const
  const tr = t(key)
  return tr === key ? kind : tr
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
  pollTimer = setInterval(() => void refresh(), 60_000)
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

watch(open, (isOpen) => {
  if (isOpen) {
    viewingId.value = null
    void refresh()
    void markRead()
  }
})
</script>

<template>
  <div ref="rootRef" class="relative">
    <button
      type="button"
      :class="[iconBtnClass, 'relative']"
      :aria-label="t('adminGatewayAlert.bellAria')"
      :aria-expanded="open"
      @click="toggle"
    >
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-5 w-5" aria-hidden="true">
        <path
          d="M10 2a6 6 0 00-6 6v3.586l-.707.707A1 1 0 004 14h12a1 1 0 00.707-1.707L16 11.586V8a6 6 0 00-6-6zM10 18a3 3 0 01-3-3h6a3 3 0 01-3 3z"
        />
      </svg>
      <span
        v-if="unreadCount > 0"
        class="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-rose-500 px-1 text-[10px] font-bold text-white"
      >
        {{ unreadCount > 99 ? '99+' : unreadCount }}
      </span>
    </button>

    <Teleport to="body">
      <div
        v-if="open && isCompact"
        class="fixed inset-0 z-[120] bg-black/50 backdrop-blur-sm"
        aria-hidden="true"
        @click="closePanel"
      />
    </Teleport>

    <div
      v-if="open"
      class="absolute right-0 z-[121] mt-2 w-[min(100vw-2rem,22rem)] overflow-hidden rounded-xl border border-slate-800 bg-slate-950/95 shadow-2xl backdrop-blur-md"
      :class="isCompact ? 'fixed left-4 right-4 top-16 mx-auto max-w-md' : ''"
    >
      <div class="border-b border-slate-800 px-4 py-3">
        <p class="text-sm font-semibold text-slate-100">{{ t('adminGatewayAlert.title') }}</p>
        <p class="mt-0.5 text-[11px] text-slate-500">{{ t('adminGatewayAlert.subtitle') }}</p>
      </div>

      <ThLoadingState v-if="loadingUi" class="py-8" />

      <div v-else-if="err" class="px-4 py-6 text-center text-xs text-rose-300">{{ err }}</div>

      <div v-else-if="!displayItem" class="px-4 py-8 text-center text-xs text-slate-500">
        {{ t('adminGatewayAlert.empty') }}
      </div>

      <template v-else>
        <div class="max-h-[50vh] overflow-y-auto thin-scrollbar px-4 py-3">
          <p class="text-xs font-medium text-slate-200">{{ displayItem.title }}</p>
          <p class="mt-1 text-[11px] text-slate-500">
            {{ kindLabel(displayItem.event_kind) }}
            <span v-if="displayItem.status_code"> · HTTP {{ displayItem.status_code }}</span>
            · {{ fmtTime(displayItem.created_at) }}
          </p>
          <p v-if="displayItem.summary" class="mt-2 whitespace-pre-wrap text-xs leading-relaxed text-slate-400">
            {{ displayItem.summary }}
          </p>
        </div>
        <ul v-if="otherItems.length" class="border-t border-slate-800 px-2 py-2">
          <li v-for="a in otherItems" :key="a.id">
            <button
              type="button"
              class="w-full rounded-lg px-2 py-2 text-left text-xs text-slate-400 transition hover:bg-slate-900/80 hover:text-slate-200"
              :class="viewingId === a.id ? 'bg-amber-500/10 ring-1 ring-amber-500/30' : ''"
              @click="selectItem(a.id)"
            >
              <span class="font-medium text-slate-300">{{ a.title }}</span>
              <span class="mt-0.5 block text-[10px] text-slate-500">{{ fmtTime(a.created_at) }}</span>
            </button>
          </li>
        </ul>
      </template>
    </div>
  </div>
</template>
