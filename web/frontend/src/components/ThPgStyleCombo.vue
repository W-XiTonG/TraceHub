<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useId, watch } from 'vue'

export type ThPgStyleComboOption = { value: string; label: string }

const props = withDefaults(
  defineProps<{
    modelValue: string
    options: ThPgStyleComboOption[]
    disabled?: boolean
    /** 无障碍名称（如「匹配方式」） */
    ariaLabel?: string
    /** 无匹配项时触发按钮展示 */
    emptyLabel?: string
  }>(),
  {
    disabled: false,
    ariaLabel: '',
    emptyLabel: '',
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
  change: []
}>()

const uid = useId()
const triggerId = `th-pg-combo-tr-${uid}`
const listboxId = `th-pg-combo-lb-${uid}`

const open = ref(false)
const wrapRef = ref<HTMLElement | null>(null)
const panelRef = ref<HTMLElement | null>(null)
const panelStyle = ref<Record<string, string>>({})

const currentLabel = computed(() => {
  const hit = props.options.find((o) => o.value === props.modelValue)
  if (hit) return hit.label
  if (props.modelValue) return props.modelValue
  return props.emptyLabel || ''
})

function close() {
  open.value = false
}

function updatePanelPosition() {
  const wrap = wrapRef.value
  if (!wrap) return
  const rect = wrap.getBoundingClientRect()
  const gap = 6
  const pad = 12
  const vh = window.innerHeight
  const spaceBelow = vh - rect.bottom - gap - pad
  const spaceAbove = rect.top - gap - pad
  const preferBelow = spaceBelow >= 120 || spaceBelow >= spaceAbove
  const maxH = Math.min(220, Math.max(80, preferBelow ? spaceBelow : spaceAbove))
  if (preferBelow) {
    panelStyle.value = {
      position: 'fixed',
      left: `${rect.left}px`,
      width: `${rect.width}px`,
      top: `${rect.bottom + gap}px`,
      maxHeight: `${maxH}px`,
    }
  } else {
    panelStyle.value = {
      position: 'fixed',
      left: `${rect.left}px`,
      width: `${rect.width}px`,
      bottom: `${vh - rect.top + gap}px`,
      maxHeight: `${maxH}px`,
    }
  }
}

function toggle() {
  if (props.disabled || !props.options.length) return
  open.value = !open.value
}

function pick(value: string) {
  if (value !== props.modelValue) {
    emit('update:modelValue', value)
    emit('change')
  }
  close()
}

function onDocMousedown(ev: MouseEvent) {
  if (!open.value) return
  const t = ev.target as Node
  const wrap = wrapRef.value
  const panel = panelRef.value
  if (wrap?.contains(t) || panel?.contains(t)) return
  close()
}

function onDocKeydown(ev: KeyboardEvent) {
  if (ev.key === 'Escape' && open.value) close()
}

function onWinReposition() {
  if (open.value) updatePanelPosition()
}

watch(open, async (isOpen) => {
  if (isOpen) {
    await nextTick()
    updatePanelPosition()
    window.addEventListener('resize', onWinReposition)
    window.addEventListener('scroll', onWinReposition, true)
  } else {
    window.removeEventListener('resize', onWinReposition)
    window.removeEventListener('scroll', onWinReposition, true)
  }
})

onMounted(() => {
  document.addEventListener('mousedown', onDocMousedown)
  document.addEventListener('keydown', onDocKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDocMousedown)
  document.removeEventListener('keydown', onDocKeydown)
  window.removeEventListener('resize', onWinReposition)
  window.removeEventListener('scroll', onWinReposition, true)
})
</script>

<template>
  <div ref="wrapRef" class="th-pg-model-picker min-w-0 max-w-[12rem] shrink-0">
    <button
      :id="triggerId"
      type="button"
      class="th-pg-model-picker__trigger"
      :disabled="disabled || !options.length"
      :aria-expanded="open"
      aria-haspopup="listbox"
      :aria-controls="listboxId"
      :aria-label="ariaLabel || undefined"
      @click="toggle"
    >
      <span class="th-pg-model-picker__value min-w-0 flex-1 truncate text-left text-[13px] leading-snug">{{ currentLabel }}</span>
      <svg
        xmlns="http://www.w3.org/2000/svg"
        viewBox="0 0 20 20"
        fill="currentColor"
        class="th-pg-model-picker__chev"
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
      <div
        v-show="open"
        ref="panelRef"
        class="th-pg-model-picker__panel"
        role="region"
        :aria-label="ariaLabel || undefined"
        :style="panelStyle"
      >
        <div
          :id="listboxId"
          class="th-pg-model-picker__options thin-scrollbar"
          role="listbox"
          :aria-labelledby="triggerId"
        >
          <button
            v-for="opt in options"
            :key="opt.value"
            type="button"
            role="option"
            :aria-selected="modelValue === opt.value"
            class="th-pg-model-picker__option"
            :class="{ 'th-pg-model-picker__option--active': modelValue === opt.value }"
            @click="pick(opt.value)"
          >
            <span class="th-pg-model-picker__opt-text">{{ opt.label }}</span>
          </button>
        </div>
      </div>
    </Teleport>
  </div>
</template>
