<script setup lang="ts">
import type { BrandLibIcon } from '../../composables/useAdminSiteSettings'

defineProps<{
  label: string
  hint: string
  url: string
  previewSrc: string
  previewKey: string
  uploading: boolean
  saving: boolean
  uploadLabel: string
  uploadingLabel: string
  previewLabel: string
  libraryLabel: string
  libraryHint: string
  libraryEmpty: string
  deleteTitle: string
  iconsForUi: BrandLibIcon[]
  iconsUploadedCount: number
  deletingFilename: string | null
  isTileSelected: (ico: BrandLibIcon) => boolean
  iconTitle: (ico: BrandLibIcon) => string
}>()

const emit = defineEmits<{
  'update:url': [value: string]
  upload: [ev: Event]
  select: [ico: BrandLibIcon]
  delete: [ico: BrandLibIcon]
}>()
</script>

<template>
  <div>
    <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{ label }}</label>
    <div class="flex max-w-xl flex-wrap items-center gap-2">
      <input
        :value="url"
        type="text"
        class="th-input min-w-[12rem] flex-1 font-mono text-sm"
        autocomplete="off"
        @input="emit('update:url', ($event.target as HTMLInputElement).value)"
      />
      <label class="th-btn-secondary inline-flex cursor-pointer items-center px-3 py-2 text-xs font-medium select-none">
        <input
          type="file"
          accept="image/png,image/jpeg,image/gif,image/webp"
          class="sr-only"
          :disabled="uploading || saving"
          @change="emit('upload', $event)"
        />
        {{ uploading ? uploadingLabel : uploadLabel }}
      </label>
    </div>
    <div class="mt-3">
      <p class="text-[11px] font-medium text-slate-400">{{ libraryLabel }}</p>
      <p class="mt-0.5 text-[11px] text-slate-500">{{ libraryHint }}</p>
      <div class="mt-2 flex max-w-xl flex-wrap gap-2">
        <div
          v-for="ico in iconsForUi"
          :key="ico.is_builtin_default ? '__builtin__' : ico.url"
          class="relative h-11 w-11 shrink-0"
        >
          <button
            type="button"
            :title="iconTitle(ico)"
            class="h-11 w-11 overflow-hidden rounded-md border bg-black/25 p-0 transition-colors hover:border-white/25"
            :class="isTileSelected(ico) ? 'border-cyan-500 ring-2 ring-cyan-500/30' : 'border-white/10'"
            @click="emit('select', ico)"
          >
            <img
              v-if="!ico.is_builtin_no_image"
              :src="ico.url.trim()"
              alt=""
              class="h-full w-full object-contain"
            />
            <span
              v-else
              class="flex h-full w-full items-center justify-center text-[9px] leading-tight text-slate-500"
            >—</span>
          </button>
          <button
            v-if="!ico.is_builtin_default && ico.filename"
            type="button"
            :title="deleteTitle"
            :disabled="deletingFilename !== null || saving || uploading"
            class="absolute -right-1 -top-1 z-10 flex h-[18px] w-[18px] items-center justify-center rounded-full border border-red-500/40 bg-red-950/90 text-[11px] leading-none font-semibold text-red-100 shadow-sm hover:bg-red-900/95 disabled:cursor-not-allowed disabled:opacity-40"
            @click.stop="emit('delete', ico)"
          >
            ×
          </button>
        </div>
      </div>
      <p v-if="iconsUploadedCount === 0" class="mt-2 text-[11px] text-slate-600">{{ libraryEmpty }}</p>
    </div>
    <p class="mt-2 flex items-center gap-2 text-[11px] text-slate-500">
      <span>{{ previewLabel }}</span>
      <img
        v-if="previewSrc"
        :key="previewKey"
        :src="previewSrc"
        alt=""
        class="h-8 w-8 rounded border border-white/10 bg-black/20 object-contain"
      />
      <span v-else class="inline-flex h-8 w-8 items-center justify-center rounded border border-white/10 bg-black/20 text-[10px] text-slate-500">—</span>
    </p>
    <p class="mt-1 text-[11px] text-slate-500">{{ hint }}</p>
  </div>
</template>
