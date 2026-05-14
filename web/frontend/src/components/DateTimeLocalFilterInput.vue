<script setup lang="ts">
import { computed, ref } from 'vue'
import { openDateTimePickerField } from '../utils/datetimeLocal'

const model = defineModel<string>({ default: '' })

const props = withDefaults(
  defineProps<{
    /** 与外层 `<label for>` 一致，便于点文字也能聚焦 */
    inputId: string
    /** 无障碍：读屏器朗读控件用途 */
    ariaLabel: string
    /** 原生 `min`，例如自定义过期时间下限 */
    min?: string
    /** 追加到外层容器上的 class（如 `max-w-none` 占满父级） */
    wrapperClass?: string
    /** 追加到 input 上的 class */
    inputClass?: string
    /**
     * 无值时展示的格式提示（覆盖 Blink/WebKit 在中文环境下空态占位符混用「日」的问题）。
     * 与常见空态骨架一致：`yyyy/mm/dd` + 时间。
     */
    emptyDisplay?: string
    /** filter：筛选条整格可点日历；field：仅原生日历图标（弹窗表单等） */
    variant?: 'filter' | 'field'
  }>(),
  {
    wrapperClass: '',
    inputClass: '',
    emptyDisplay: 'yyyy/mm/dd --:--',
    variant: 'filter',
  },
)

const inputRef = ref<HTMLInputElement | null>(null)

const showEmptyHint = computed(() => !String(model.value ?? '').trim())

const pickerSurfaceClass = computed(() =>
  props.variant === 'filter' ? 'th-datetime-local th-datetime-local--filter' : 'th-datetime-local',
)

function openPicker() {
  openDateTimePickerField(inputRef.value)
}

function onDateTimeChange() {
  window.setTimeout(() => {
    try {
      inputRef.value?.blur()
    } catch {
      /* ignore */
    }
  }, 0)
}
</script>

<template>
  <div
    :class="[
      'th-datetime-local-wrap group relative isolate h-10 w-full max-w-[11.5rem] rounded-[0.5rem]',
      props.wrapperClass,
    ]"
  >
    <span
      v-show="showEmptyHint"
      class="th-datetime-local-empty-hint pointer-events-none absolute inset-0 z-0 flex items-center pl-3.5 pr-2 text-sm leading-tight text-slate-500"
      aria-hidden="true"
    >
      {{ props.emptyDisplay }}
    </span>
    <input
      :id="props.inputId"
      ref="inputRef"
      v-model="model"
      type="datetime-local"
      step="60"
      tabindex="0"
      :min="props.min"
      :class="[
        'th-input relative z-10 h-10 w-full max-w-full pr-2 text-sm leading-tight',
        pickerSurfaceClass,
        showEmptyHint ? 'opacity-0 shadow-none focus:border-transparent focus:shadow-none' : '',
        props.inputClass,
      ]"
      :aria-label="props.ariaLabel"
      @keydown.enter.prevent="openPicker"
      @keydown.space.prevent="openPicker"
      @change="onDateTimeChange"
    />
  </div>
</template>
