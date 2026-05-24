<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import type {
  PlaygroundModelRow,
  PlaygroundRouterHealth,
  PlaygroundServiceStatus,
} from '../../api/playgroundGateway'
import PlaygroundModelMetaPills, { type PgMetaCell } from './PlaygroundModelMetaPills.vue'

const props = defineProps<{
  models: PlaygroundModelRow[]
  model: string
  open: boolean
  filteredModels: PlaygroundModelRow[]
  selectedDisplayId: string
  selectedModelRow?: PlaygroundModelRow | null
  showSelectedModelMeta?: boolean
  playgroundRegistryIds?: Set<string> | null
  isCallable: (m: { callable?: boolean }) => boolean
  catalogOnlyTitle: string
  searchQuery?: string
  modelKindLabel: (k: string) => string
  serviceStatusCell: (st: PlaygroundServiceStatus) => PgMetaCell
  routerHealthCell: (h: PlaygroundRouterHealth) => PgMetaCell
  registryCell: (m: PlaygroundModelRow) => PgMetaCell
}>()

const emit = defineEmits<{
  'update:model': [id: string]
  'update:open': [open: boolean]
  'update:searchQuery': [q: string]
  pick: [id: string]
}>()

const wrap = ref<HTMLElement | null>(null)
const panel = ref<HTMLElement | null>(null)
const panelStyle = ref<Record<string, string>>({})
const searchInput = ref<HTMLInputElement | null>(null)

const metaProps = computed(() => ({
  registryIds: props.playgroundRegistryIds,
  modelKindLabel: props.modelKindLabel,
  serviceStatusCell: props.serviceStatusCell,
  routerHealthCell: props.routerHealthCell,
  registryCell: props.registryCell,
}))

const search = computed({
  get: () => props.searchQuery ?? '',
  set: (v: string) => emit('update:searchQuery', v),
})

const selectedCallable = computed(() =>
  props.selectedModelRow ? props.isCallable(props.selectedModelRow) : true,
)

const triggerTitle = computed(() => {
  const id = props.selectedDisplayId
  if (!props.showSelectedModelMeta || !props.selectedModelRow) return id
  const parts: string[] = [id]
  const row = props.selectedModelRow
  if (!props.isCallable(row)) {
    parts.push(props.catalogOnlyTitle)
  } else {
    if (row.serviceStatus) parts.push(props.serviceStatusCell(row.serviceStatus).text)
    if (row.routerHealth) parts.push(props.routerHealthCell(row.routerHealth).text)
    if (props.playgroundRegistryIds?.has(row.id)) parts.push(props.registryCell(row).text)
  }
  return parts.join(' · ')
})

function close() {
  emit('update:open', false)
  emit('update:searchQuery', '')
}

function updatePanelPosition() {
  const w = wrap.value
  if (!w) return
  const rect = w.getBoundingClientRect()
  const gap = 6
  const pad = 12
  const vh = window.innerHeight
  const spaceBelow = vh - rect.bottom - gap - pad
  const maxH = Math.min(360, Math.max(120, spaceBelow))
  panelStyle.value = {
    position: 'fixed',
    left: `${rect.left}px`,
    width: `${Math.max(rect.width, 320)}px`,
    top: `${rect.bottom + gap}px`,
    maxHeight: `${maxH}px`,
  }
}

function toggle() {
  if (!props.models.length) return
  emit('update:open', !props.open)
}

function onPick(id: string) {
  emit('pick', id)
  emit('update:model', id)
  close()
}

function onOptionPointer(id: string, callable: boolean, ev: Event) {
  if (!callable) return
  ev.preventDefault()
  ev.stopPropagation()
  onPick(id)
}

function onDocMousedown(ev: MouseEvent) {
  if (!props.open) return
  const t = ev.target as Node
  if (wrap.value?.contains(t) || panel.value?.contains(t)) return
  close()
}

watch(
  () => props.open,
  async (open) => {
    if (open) {
      await nextTick()
      searchInput.value?.focus()
      updatePanelPosition()
      document.addEventListener('mousedown', onDocMousedown)
      window.addEventListener('resize', updatePanelPosition)
      window.addEventListener('scroll', updatePanelPosition, true)
    } else {
      document.removeEventListener('mousedown', onDocMousedown)
      window.removeEventListener('resize', updatePanelPosition)
      window.removeEventListener('scroll', updatePanelPosition, true)
    }
  },
)

onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDocMousedown)
  window.removeEventListener('resize', updatePanelPosition)
  window.removeEventListener('scroll', updatePanelPosition, true)
})
</script>

<template>
  <div ref="wrap" class="th-pg-v2-model-select th-pg-v2-model-select--fit th-pg-model-picker min-w-0">
    <button
      type="button"
      class="th-pg-model-picker__trigger th-pg-v2-model-picker__trigger--inline"
      :disabled="!models.length"
      :aria-expanded="open"
      aria-haspopup="listbox"
      :title="triggerTitle"
      @click="toggle"
    >
      <div class="th-pg-v2-model-picker__main">
        <span class="th-pg-model-picker__value">{{ selectedDisplayId }}</span>
        <PlaygroundModelMetaPills
          v-if="showSelectedModelMeta && selectedModelRow"
          layout="inline"
          variant="triggerSummary"
          :row="selectedModelRow"
          :callable="selectedCallable"
          v-bind="metaProps"
        />
      </div>
      <svg
        xmlns="http://www.w3.org/2000/svg"
        viewBox="0 0 20 20"
        fill="currentColor"
        class="th-pg-model-picker__chev shrink-0"
        :class="{ 'th-pg-model-picker__chev--open': open }"
        aria-hidden="true"
      >
        <path
          fill-rule="evenodd"
          d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.94a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z"
          clip-rule="evenodd"
        />
      </svg>
    </button>
    <Teleport to="body">
      <div v-show="open" ref="panel" class="th-pg-model-picker__panel" role="listbox" :style="panelStyle">
        <div class="th-pg-model-picker__search-wrap">
          <input
            ref="searchInput"
            v-model="search"
            type="search"
            autocomplete="off"
            class="th-pg-model-picker__search"
            :placeholder="$t('playground.modelSearchPh')"
          />
        </div>
        <div class="th-pg-model-picker__options thin-scrollbar">
          <button
            v-for="m in filteredModels"
            :key="m.id"
            type="button"
            role="option"
            :aria-selected="model === m.id"
            :disabled="!isCallable(m)"
            class="th-pg-model-picker__option"
            :class="{
              'th-pg-model-picker__option--active': model === m.id,
              'th-pg-model-picker__option--disabled': !isCallable(m),
            }"
            :title="!isCallable(m) ? catalogOnlyTitle : undefined"
            @mousedown="onOptionPointer(m.id, isCallable(m), $event)"
          >
            <span class="th-pg-model-picker__opt-id">{{ m.id }}</span>
            <PlaygroundModelMetaPills
              layout="column"
              :row="m"
              :callable="isCallable(m)"
              show-catalog-badge
              v-bind="metaProps"
            />
          </button>
          <p v-if="!filteredModels.length" class="th-pg-model-picker__empty">
            {{ $t('playground.modelSearchEmpty') }}
          </p>
        </div>
      </div>
    </Teleport>
  </div>
</template>
