<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'

const { t } = useI18n()

type Row = { id: number; email: string; balance: number; status: number; group_name?: string }

const items = ref<Row[]>([])
const deltaByUser = ref<Record<number, string>>({})
const filterText = ref('')
const compact = ref(false)
const selectedIds = ref<number[]>([])
const flash = ref('')
const err = ref('')

const filteredItems = computed(() => {
  const q = filterText.value.trim().toLowerCase()
  if (!q) return items.value
  return items.value.filter((u) => {
    const idStr = String(u.id)
    const email = (u.email || '').toLowerCase()
    return email.includes(q) || idStr.includes(q)
  })
})

const allFilteredSelected = computed(() => {
  const f = filteredItems.value
  if (!f.length) return false
  return f.every((u) => selectedIds.value.includes(u.id))
})

function toggleSelectAll(checked: boolean) {
  if (checked) {
    const set = new Set(selectedIds.value)
    for (const u of filteredItems.value) set.add(u.id)
    selectedIds.value = [...set]
  } else {
    const drop = new Set(filteredItems.value.map((u) => u.id))
    selectedIds.value = selectedIds.value.filter((id) => !drop.has(id))
  }
}

function toggleRow(id: number) {
  if (selectedIds.value.includes(id)) {
    selectedIds.value = selectedIds.value.filter((x) => x !== id)
  } else {
    selectedIds.value = [...selectedIds.value, id]
  }
}

async function load() {
  err.value = ''
  try {
    const { data } = await http.get<Envelope<{ items: Row[] }>>('/api/admin/users')
    if (data.code !== 0) throw new Error(data.message || t('adminCommon.loadFail'))
    items.value = data.data.items
    const idSet = new Set(items.value.map((x) => x.id))
    selectedIds.value = selectedIds.value.filter((id) => idSet.has(id))
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('adminCommon.loadFail')
  }
}

function resetFilter() {
  filterText.value = ''
  void load()
}

async function adjust(id: number) {
  const raw = deltaByUser.value[id]
  const delta = parseInt(raw || '0', 10)
  if (!delta) return
  err.value = ''
  try {
    await http.post(`/api/admin/users/${id}/balance`, { delta })
    deltaByUser.value[id] = ''
    await load()
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('adminCommon.requestFail')
  }
}

function statusBadge(u: Row) {
  return u.status === 1
    ? 'border-emerald-500/35 bg-emerald-500/12 text-emerald-200'
    : 'border-slate-500/35 bg-slate-500/10 text-slate-400'
}

function statusLabel(u: Row) {
  return u.status === 1 ? t('adminUsers.statusActive') : t('adminUsers.statusInactive')
}

async function copyEmails() {
  const set = new Set(selectedIds.value)
  const emails = items.value.filter((u) => set.has(u.id)).map((u) => u.email).filter(Boolean)
  if (!emails.length) {
    err.value = t('adminUsers.nothingSelected')
    return
  }
  err.value = ''
  try {
    await navigator.clipboard.writeText(emails.join('\n'))
    flash.value = t('adminUsers.copied')
    window.setTimeout(() => {
      if (flash.value === t('adminUsers.copied')) flash.value = ''
    }, 2200)
  } catch {
    flash.value = t('adminUsers.copyFail')
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <div>
    <div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
      <div class="flex min-w-0 items-start gap-3">
        <span class="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-sky-500/12 text-sky-300 ring-1 ring-sky-500/25">
          <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M15 19.128a9.38 9.38 0 002.625.372 9.337 9.337 0 004.121-.952 4.125 4.125 0 00-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 018.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0111.964-3.07M12 6.375a3.375 3.375 0 11-6.75 0 3.375 3.375 0 016.75 0zm8.25 2.25a2.625 2.625 0 11-5.25 0 2.625 2.625 0 015.25 0z"
            />
          </svg>
        </span>
        <div class="min-w-0">
          <p class="text-[11px] font-medium uppercase tracking-wider text-slate-500">{{ t('adminUsers.pageKicker') }}</p>
          <h1 class="text-2xl font-bold tracking-tight text-slate-100">{{ t('adminUsers.title') }}</h1>
          <p class="mt-1 max-w-2xl text-sm leading-relaxed text-slate-500">{{ t('adminUsers.desc') }}</p>
        </div>
      </div>
      <button
        type="button"
        class="shrink-0 self-start text-[13px] text-cyan-400/90 underline-offset-4 hover:text-cyan-300 hover:underline"
        @click="compact = !compact"
      >
        {{ compact ? t('adminUsers.standardList') : t('adminUsers.compactList') }}
      </button>
    </div>

    <div class="mb-6 flex flex-col gap-3 rounded-xl border border-white/[0.06] bg-black/20 px-4 py-3 sm:flex-row sm:flex-wrap sm:items-center">
      <input
        v-model="filterText"
        type="search"
        :placeholder="t('adminUsers.searchPh')"
        class="min-w-[12rem] flex-1 rounded-lg border border-slate-600/80 bg-slate-950/80 px-3 py-2 text-sm text-slate-100 placeholder:text-slate-600 sm:max-w-xs"
      />
      <button type="button" class="rounded-lg bg-cyan-600 px-3 py-2 text-sm font-medium text-white hover:bg-cyan-500" @click="load">
        {{ t('adminCommon.query') }}
      </button>
      <button type="button" class="rounded-lg border border-slate-600 px-3 py-2 text-sm text-slate-400 hover:bg-white/5" @click="resetFilter">
        {{ t('adminUsers.resetFilter') }}
      </button>
      <button type="button" class="rounded-lg border border-slate-600 px-3 py-2 text-sm text-slate-200 hover:bg-white/5" @click="copyEmails">
        {{ t('adminUsers.copyEmails') }}
      </button>
    </div>

    <p v-if="flash" class="mb-3 text-sm text-emerald-400">{{ flash }}</p>
    <p v-if="err" class="mb-4 text-sm text-red-400">{{ err }}</p>

    <div class="overflow-x-auto rounded-xl border border-slate-800/80">
      <table class="w-full min-w-[640px] text-left" :class="compact ? 'text-xs' : 'text-sm'">
        <thead class="border-b border-slate-800 bg-slate-950/50 text-slate-500">
          <tr>
            <th class="w-10 px-3 py-2.5" :class="compact ? 'py-1.5' : ''">
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-slate-600 bg-slate-950 text-cyan-500"
                :checked="allFilteredSelected"
                @change="toggleSelectAll(($event.target as HTMLInputElement).checked)"
              />
            </th>
            <th class="px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('adminUsers.thId') }}</th>
            <th class="px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('adminUsers.thEmail') }}</th>
            <th class="px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('adminUsers.thStatus') }}</th>
            <th class="px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('adminUsers.thGroup') }}</th>
            <th class="px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('adminUsers.thBalance') }}</th>
            <th class="px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('adminUsers.thDelta') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800/90">
          <tr v-for="u in filteredItems" :key="u.id" class="hover:bg-white/[0.02]">
            <td class="px-3" :class="compact ? 'py-1.5' : 'py-2.5'">
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-slate-600 bg-slate-950 text-cyan-500"
                :checked="selectedIds.includes(u.id)"
                @change="toggleRow(u.id)"
              />
            </td>
            <td class="px-3 font-mono text-slate-400" :class="compact ? 'py-1.5' : 'py-2.5'">{{ u.id }}</td>
            <td class="px-3 text-slate-200" :class="compact ? 'py-1.5' : 'py-2.5'">{{ u.email }}</td>
            <td class="px-3" :class="compact ? 'py-1.5' : 'py-2.5'">
              <span class="inline-flex rounded-full border px-2 py-0.5 text-[11px] font-medium" :class="statusBadge(u)">
                {{ statusLabel(u) }}
              </span>
            </td>
            <td class="px-3 text-slate-400" :class="compact ? 'py-1.5' : 'py-2.5'">{{ u.group_name || t('adminCommon.dash') }}</td>
            <td class="px-3 font-mono text-slate-300" :class="compact ? 'py-1.5' : 'py-2.5'">{{ u.balance }}</td>
            <td class="px-3" :class="compact ? 'py-1.5' : 'py-2.5'">
              <div class="flex flex-wrap items-center gap-2">
                <input
                  v-model="deltaByUser[u.id]"
                  type="number"
                  :placeholder="t('adminUsers.deltaPh')"
                  class="w-24 rounded border border-slate-700 bg-slate-950 px-2 py-1 text-xs"
                />
                <button type="button" class="text-[11px] text-cyan-400 hover:underline" @click="adjust(u.id)">{{ t('adminUsers.apply') }}</button>
              </div>
            </td>
          </tr>
          <tr v-if="items.length && !filteredItems.length">
            <td colspan="7" class="px-3 py-8 text-center text-slate-500">{{ t('adminUsers.noMatch') }}</td>
          </tr>
          <tr v-if="!items.length">
            <td colspan="7" class="px-3 py-8 text-center text-slate-500">{{ t('adminCommon.dash') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
