<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { channelRefLabel, type AdminModelChannelRef } from '../../utils/adminModelChannelMap'

const props = defineProps<{
  open: boolean
  modelId: string
  channels: AdminModelChannelRef[]
  selectedChannelId: number | ''
}>()

const emit = defineEmits<{
  'update:selectedChannelId': [value: number | '']
  confirm: []
  cancel: []
}>()

const { t } = useI18n()

const canConfirm = computed(() => {
  const id = props.selectedChannelId
  return typeof id === 'number' && id > 0
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-[10000] flex cursor-pointer items-center justify-center bg-black/60 p-4 backdrop-blur-sm"
      @click.self="emit('cancel')"
    >
      <div
        class="max-h-[min(85vh,28rem)] w-full max-w-md cursor-default overflow-hidden rounded-2xl border border-slate-700 bg-slate-900 shadow-2xl"
        role="dialog"
        aria-modal="true"
        aria-labelledby="model-channel-pick-title"
        @click.stop
      >
        <div class="border-b border-slate-800 px-5 py-4">
          <h4 id="model-channel-pick-title" class="text-base font-semibold text-white">
            {{ t('adminModelChannelPick.title') }}
          </h4>
          <p class="mt-1.5 text-xs leading-relaxed text-slate-400">
            {{ t('adminModelChannelPick.intro', { model: modelId }) }}
          </p>
          <p class="mt-1 text-[11px] leading-relaxed text-slate-500">{{ t('adminModelChannelPick.hint') }}</p>
        </div>
        <ul class="thin-scrollbar max-h-[min(50vh,16rem)] overflow-y-auto divide-y divide-slate-800">
          <li v-for="ch in channels" :key="ch.id">
            <label
              class="flex cursor-pointer items-start gap-3 px-5 py-3 hover:bg-slate-800/50"
              :class="selectedChannelId === ch.id ? 'bg-cyan-500/10' : ''"
            >
              <input
                type="radio"
                class="mt-0.5"
                :checked="selectedChannelId === ch.id"
                @change="emit('update:selectedChannelId', ch.id)"
              />
              <span class="min-w-0 flex-1 text-sm">
                <span class="font-mono text-slate-200">{{ channelRefLabel(ch) }}</span>
                <span
                  v-if="!ch.healthy"
                  class="ml-2 text-[10px] font-medium uppercase tracking-wide text-amber-400/90"
                >{{ t('adminModelChannelPick.unhealthy') }}</span>
              </span>
            </label>
          </li>
        </ul>
        <div class="flex flex-col-reverse gap-2 border-t border-slate-800 px-5 py-3 sm:flex-row sm:justify-end">
          <button
            type="button"
            class="rounded-lg border border-slate-600 px-4 py-2 text-xs text-slate-300 hover:bg-slate-800"
            @click="emit('cancel')"
          >
            {{ t('adminCommon.cancel') }}
          </button>
          <button
            type="button"
            class="rounded-lg bg-cyan-600 px-4 py-2 text-xs font-medium text-white hover:bg-cyan-500 disabled:opacity-40"
            :disabled="!canConfirm"
            @click="emit('confirm')"
          >
            {{ t('adminModelChannelPick.confirm') }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
