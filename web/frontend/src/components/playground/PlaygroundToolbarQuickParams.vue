<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PlaygroundPlayMode } from '../../types/playground'

const props = defineProps<{
  playMode: PlaygroundPlayMode
  imageSizeOptions: readonly string[]
  videoSizeOptions: readonly string[]
  fmtNum: (n: number) => string
}>()

const temperature = defineModel<number>('temperature', { default: 1 })
const useTemp = defineModel<boolean>('useTemp', { default: true })
const imageGenSize = defineModel<string>('imageGenSize', { default: '' })
const videoSeconds = defineModel<string>('videoSeconds', { default: '' })
const videoSize = defineModel<string>('videoSize', { default: '' })

const { t } = useI18n()

const showChat = computed(() => props.playMode === 'chat')
const showImage = computed(() => props.playMode === 'image')
const showVideo = computed(() => props.playMode === 'video')

function onTempInput() {
  useTemp.value = true
}
</script>

<template>
  <div class="th-pg-v2-toolbar__center" role="group" :aria-label="t('playground.quickParamsAria')">
    <div v-if="showChat" class="th-pg-v2-quick-param th-pg-v2-quick-param--temp">
      <span class="th-pg-v2-quick-param__label">{{ t('playground.temp') }}</span>
      <input
        v-model.number="temperature"
        type="range"
        min="0"
        max="2"
        step="0.05"
        class="th-pg-v2-quick-param__range"
        :aria-valuenow="temperature"
        @input="onTempInput"
      />
      <span class="th-pg-v2-quick-param__value" aria-hidden="true">{{ fmtNum(temperature) }}</span>
    </div>

    <label v-else-if="showImage" class="th-pg-v2-quick-param">
      <span class="th-pg-v2-quick-param__label">{{ t('playground.imageGenSize') }}</span>
      <select v-model="imageGenSize" class="th-pg-v2-quick-param__select">
        <option v-for="s in imageSizeOptions" :key="s" :value="s">{{ s }}</option>
      </select>
    </label>

    <template v-else-if="showVideo">
      <label class="th-pg-v2-quick-param">
        <span class="th-pg-v2-quick-param__label">{{ t('playground.videoSeconds') }}</span>
        <input
          v-model="videoSeconds"
          type="text"
          class="th-pg-v2-quick-param__input th-pg-v2-quick-param__input--short"
          :placeholder="t('playground.videoSecondsPlaceholder')"
        />
      </label>
      <label class="th-pg-v2-quick-param">
        <span class="th-pg-v2-quick-param__label">{{ t('playground.videoSize') }}</span>
        <select v-model="videoSize" class="th-pg-v2-quick-param__select">
          <option v-for="s in videoSizeOptions" :key="s" :value="s">{{ s }}</option>
        </select>
      </label>
    </template>
  </div>
</template>
