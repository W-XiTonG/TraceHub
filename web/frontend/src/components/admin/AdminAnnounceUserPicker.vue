<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../../api/http'
import ThLoadingState from '../ui/ThLoadingState.vue'
import { useSmoothLoading } from '../../composables/useSmoothLoading'

const props = defineProps<{
  modelValue: number[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: number[]]
}>()

const { t } = useI18n()

type UserRow = {
  id: number
  email: string
  group_name?: string
}

const items = ref<UserRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const filterText = ref('')
const filterApplied = ref('')
const { busy: loading, visible: loadingUi, run: runLoad } = useSmoothLoading()

const selectedIds = computed({
  get: () => props.modelValue,
  set: (ids: number[]) => emit('update:modelValue', ids),
})

const totalPages = computed(() => {
  if (total.value <= 0) return 1
  return Math.max(1, Math.ceil(total.value / pageSize))
})

const allPageSelected = computed(() => {
  if (!items.value.length) return false
  return items.value.every((u) => selectedIds.value.includes(u.id))
})

const anyPageSelected = computed(() => items.value.some((u) => selectedIds.value.includes(u.id)))

function listQueryParams(): Record<string, string | number> {
  const params: Record<string, string | number> = {
    page: page.value,
    page_size: pageSize,
  }
  const q = filterApplied.value.trim()
  if (q) {
    if (/^\d+$/.test(q)) params.user_id = q
    else params.email = q
  }
  return params
}

async function load() {
  await runLoad(async () => {
    const { data } = await http.get<
      Envelope<{ items: UserRow[]; total: number; page?: number }>
    >('/api/admin/users', { params: listQueryParams() })
    if (data.code !== 0) throw new Error(data.message || t('adminCommon.loadFail'))
    items.value = data.data?.items ?? []
    total.value = typeof data.data?.total === 'number' ? data.data.total : 0
    if (typeof data.data?.page === 'number' && data.data.page >= 1) {
      page.value = data.data.page
    }
  }).catch(() => {
    items.value = []
    total.value = 0
  })
}

function applyFilter() {
  filterApplied.value = filterText.value.trim()
  page.value = 1
  void load()
}

function resetFilter() {
  filterText.value = ''
  filterApplied.value = ''
  page.value = 1
  void load()
}

function toggleRow(id: number) {
  if (selectedIds.value.includes(id)) {
    selectedIds.value = selectedIds.value.filter((x) => x !== id)
  } else {
    selectedIds.value = [...selectedIds.value, id]
  }
}

function toggleSelectAllCurrentPage(checked: boolean) {
  if (checked) {
    const set = new Set(selectedIds.value)
    for (const u of items.value) set.add(u.id)
    selectedIds.value = [...set]
  } else {
    const drop = new Set(items.value.map((u) => u.id))
    selectedIds.value = selectedIds.value.filter((id) => !drop.has(id))
  }
}

function prevPage() {
  if (page.value > 1) {
    page.value -= 1
    void load()
  }
}

function nextPage() {
  if (page.value < totalPages.value) {
    page.value += 1
    void load()
  }
}

onMounted(() => void load())
</script>

<template>
  <div class="flex flex-col gap-2">
    <p class="text-xs text-slate-500">{{ t('adminAnnounce.userPickerHint') }}</p>
    <div class="flex flex-wrap items-center gap-2">
      <input
        v-model="filterText"
        type="search"
        class="th-input min-w-0 flex-1 text-sm"
        :placeholder="t('adminUsers.searchPh')"
        @keydown.enter.prevent="applyFilter"
      />
      <button type="button" class="th-btn-secondary shrink-0 px-3 py-1.5 text-sm" :disabled="loading" @click="applyFilter">
        {{ t('adminCommon.query') }}
      </button>
      <button type="button" class="th-ghost-hover shrink-0 rounded-lg border border-slate-600 px-3 py-1.5 text-sm text-slate-200" :disabled="loading" @click="resetFilter">
        {{ t('adminUsers.resetFilter') }}
      </button>
    </div>

    <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-slate-400">
      <span>{{ t('adminAnnounce.userPickerSelected', { n: selectedIds.length }) }}</span>
      <div class="flex gap-2">
        <button
          type="button"
          class="th-link-accent hover:underline"
          :disabled="!items.length || allPageSelected"
          @click="toggleSelectAllCurrentPage(true)"
        >
          {{ t('adminAnnounce.userPickerSelectPage') }}
        </button>
        <button
          type="button"
          class="text-slate-500 hover:underline"
          :disabled="!items.length || !anyPageSelected"
          @click="toggleSelectAllCurrentPage(false)"
        >
          {{ t('adminAnnounce.userPickerClearPage') }}
        </button>
      </div>
    </div>

    <div class="relative max-h-64 overflow-y-auto rounded-lg border border-slate-700/80">
      <ThLoadingState v-if="loadingUi && !items.length" />
      <table v-else-if="items.length" class="w-full text-left text-xs">
        <thead class="sticky top-0 bg-slate-900/95 text-slate-500">
          <tr>
            <th class="w-10 px-2 py-2" />
            <th class="px-2 py-2">{{ t('adminUsers.thId') }}</th>
            <th class="px-2 py-2">{{ t('adminUsers.thEmail') }}</th>
            <th class="hidden px-2 py-2 sm:table-cell">{{ t('adminUsers.thGroup') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="u in items"
            :key="u.id"
            class="cursor-pointer border-t border-slate-800/80 text-slate-300 hover:bg-slate-800/40"
            @click="toggleRow(u.id)"
          >
            <td class="px-2 py-2" @click.stop>
              <input
                type="checkbox"
                class="accent-[var(--th-accent)]"
                :checked="selectedIds.includes(u.id)"
                @change="toggleRow(u.id)"
              />
            </td>
            <td class="px-2 py-2 font-mono text-slate-400">{{ u.id }}</td>
            <td class="max-w-[180px] truncate px-2 py-2" :title="u.email">{{ u.email }}</td>
            <td class="hidden px-2 py-2 text-slate-500 sm:table-cell">{{ u.group_name || '—' }}</td>
          </tr>
        </tbody>
      </table>
      <p v-else-if="!loadingUi" class="px-3 py-6 text-center text-xs text-slate-500">
        {{ t('adminAnnounce.userPickerEmpty') }}
      </p>
    </div>

    <div class="flex items-center justify-end gap-2">
      <button type="button" class="th-admin-toolbar-btn text-xs" :disabled="page <= 1 || loading" @click="prevPage">
        {{ t('adminCommon.prevPage') }}
      </button>
      <span class="text-xs tabular-nums text-slate-500">{{ page }} / {{ totalPages }}</span>
      <button type="button" class="th-admin-toolbar-btn text-xs" :disabled="page >= totalPages || loading" @click="nextPage">
        {{ t('adminCommon.nextPage') }}
      </button>
    </div>
  </div>
</template>
