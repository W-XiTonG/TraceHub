<script setup lang="ts">
import { inject } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminSiteSettingsKey } from '../composables/useAdminSiteSettings'

const { t } = useI18n()
const s = inject(adminSiteSettingsKey)!
</script>

<template>
  <div class="space-y-8">
    <section class="th-glass rounded-[var(--th-radius-lg)] p-5 md:p-6">
      <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secBrand') }}</h2>
      <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secBrandHint') }}</p>
      <div class="mt-5 space-y-4">
        <div>
          <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{ t('adminSystem.lblBrandName') }}</label>
          <input v-model="s.form.brandName" type="text" class="th-input max-w-xl" autocomplete="off" />
        </div>
        <div>
          <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{ t('adminSystem.lblBrandIcon') }}</label>
          <div class="flex max-w-xl flex-wrap items-center gap-2">
            <input v-model="s.form.brandIconUrl" type="text" class="th-input min-w-[12rem] flex-1 font-mono text-sm" autocomplete="off" />
            <label class="th-btn-secondary inline-flex cursor-pointer items-center px-3 py-2 text-xs font-medium select-none">
              <input
                type="file"
                accept="image/png,image/jpeg,image/gif,image/webp"
                class="sr-only"
                :disabled="s.uploadingIcon || s.saving"
                @change="s.uploadBrandIcon"
              />
              {{ s.uploadingIcon ? t('adminSystem.brandIconUploading') : t('adminSystem.brandIconUpload') }}
            </label>
          </div>
          <div class="mt-3">
            <p class="text-[11px] font-medium text-slate-400">{{ t('adminSystem.brandIconLibrary') }}</p>
            <p class="mt-0.5 text-[11px] text-slate-500">{{ t('adminSystem.brandIconLibraryHint') }}</p>
            <div class="mt-2 flex max-w-xl flex-wrap gap-2">
              <div
                v-for="ico in s.brandIconsForUi"
                :key="ico.is_builtin_default ? '__builtin_default__' : ico.url"
                class="relative h-11 w-11 shrink-0"
              >
                <button
                  type="button"
                  :title="s.brandIconTitle(ico)"
                  class="h-11 w-11 overflow-hidden rounded-md border bg-black/25 p-0 transition-colors hover:border-white/25"
                  :class="s.isBrandIconTileSelected(ico) ? 'border-cyan-500 ring-2 ring-cyan-500/30' : 'border-white/10'"
                  @click="s.selectBrandIcon(ico)"
                >
                  <img :src="ico.url.trim()" alt="" class="h-full w-full object-contain" />
                </button>
                <button
                  v-if="!ico.is_builtin_default && ico.filename"
                  type="button"
                  :title="t('adminSystem.brandIconDelete')"
                  :disabled="s.deletingBrandFilename !== null || s.saving || s.uploadingIcon"
                  class="absolute -right-1 -top-1 z-10 flex h-[18px] w-[18px] items-center justify-center rounded-full border border-red-500/40 bg-red-950/90 text-[11px] leading-none font-semibold text-red-100 shadow-sm hover:bg-red-900/95 disabled:cursor-not-allowed disabled:opacity-40"
                  @click.stop="void s.confirmDeleteBrandIcon(ico)"
                >
                  ×
                </button>
              </div>
            </div>
            <p v-if="s.brandIcons.length === 0" class="mt-2 text-[11px] text-slate-600">{{ t('adminSystem.brandIconLibraryEmpty') }}</p>
          </div>
          <p class="mt-2 flex items-center gap-2 text-[11px] text-slate-500">
            <span>{{ t('adminSystem.brandIconPreview') }}</span>
            <img
              :key="`${s.brandIconPreviewSrc}|${s.brandPreviewNonce}`"
              :src="s.brandIconPreviewSrc"
              alt=""
              class="h-8 w-8 rounded border border-white/10 bg-black/20 object-contain"
            />
          </p>
          <p class="mt-1 text-[11px] text-slate-500">{{ t('adminSystem.hintBrandIcon') }}</p>
        </div>
      </div>
    </section>
  </div>
</template>
