<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { FAILOVER_4XX_CODE_OPTIONS } from '../../constants/failover4xx'

const props = defineProps<{
  modelValue: number[]
  mode: string
}>()

const emit = defineEmits<{
  'update:modelValue': [number[]]
}>()

const { t } = useI18n()

const showPicker = computed(() => props.mode === 'selective' || props.mode === 'custom')

const hintKey = computed(() => {
  if (props.mode === 'custom') return 'adminGroups.hintFailover4xxCodesCustom'
  if (props.mode === 'selective') return 'adminGroups.hintFailover4xxCodesSelective'
  return ''
})

function isChecked(code: number): boolean {
  return props.modelValue.includes(code)
}

function toggle(code: number) {
  const set = new Set(props.modelValue)
  if (set.has(code)) set.delete(code)
  else set.add(code)
  emit('update:modelValue', [...set].sort((a, b) => a - b))
}
</script>

<template>
  <div v-if="showPicker" class="mt-2 space-y-2 rounded-lg border border-slate-800 bg-slate-950/50 p-3">
    <p class="text-[11px] font-medium text-slate-400">{{ t('adminGroups.lblFailover4xxCodes') }}</p>
    <p v-if="hintKey" class="text-[10px] leading-relaxed text-slate-500">{{ t(hintKey) }}</p>
    <div class="grid gap-2 sm:grid-cols-2">
      <label
        v-for="code in FAILOVER_4XX_CODE_OPTIONS"
        :key="code"
        class="flex cursor-pointer items-start gap-2 rounded border border-slate-800/80 px-2 py-1.5 text-xs text-slate-300 hover:border-slate-700"
      >
        <input
          type="checkbox"
          class="mt-0.5"
          :checked="isChecked(code)"
          @change="toggle(code)"
        />
        <span>
          <span class="font-mono font-medium text-slate-200">{{ code }}</span>
          <span class="mt-0.5 block text-[10px] text-slate-500">{{ t(`adminGroups.failover4xxCode${code}`) }}</span>
        </span>
      </label>
    </div>
    <p class="text-[10px] text-slate-600">{{ t('adminGroups.hintFailover4xxCodesAlways') }}</p>
  </div>
</template>
