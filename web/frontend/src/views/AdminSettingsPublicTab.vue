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
      <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secApiPublic') }}</h2>
      <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secApiPublicHint') }}</p>
      <div class="mt-4 max-w-xl">
        <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
          t('adminSystem.lblApiPublicBaseUrl')
        }}</label>
        <input
          v-model="s.form.apiPublicBaseUrl"
          type="url"
          class="th-input w-full font-mono text-sm"
          autocomplete="off"
          placeholder="https://api.example.com"
        />
        <p class="mt-2 text-[11px] leading-relaxed text-slate-500">{{ t('adminSystem.hintApiPublicBaseUrl') }}</p>
      </div>
    </section>

    <section class="th-glass rounded-[var(--th-radius-lg)] p-5 md:p-6">
      <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secBeianFooter') }}</h2>
      <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secBeianFooterHint') }}</p>
      <div class="mt-4 max-w-xl">
        <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
          t('adminSystem.lblBeianFooterText')
        }}</label>
        <textarea
          v-model="s.form.beianFooterText"
          rows="4"
          class="th-input w-full resize-y font-mono text-sm"
          :placeholder="t('adminSystem.phBeianFooterText')"
          autocomplete="off"
        />
      </div>
    </section>

    <section class="th-glass rounded-[var(--th-radius-lg)] p-5 md:p-6">
      <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secContact') }}</h2>
      <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secContactHint') }}</p>
      <div class="mt-4 max-w-xl space-y-3">
        <div>
          <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
            t('adminSystem.lblContactTitle')
          }}</label>
          <input v-model="s.contactTitle" type="text" class="th-input w-full" autocomplete="off" :placeholder="t('adminSystem.phContactTitle')" />
        </div>
        <div>
          <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
            t('adminSystem.lblContactEmails')
          }}</label>
          <textarea
            v-model="s.contactEmailsText"
            rows="3"
            class="th-input w-full resize-y font-mono text-sm"
            :placeholder="t('adminSystem.phContactEmails')"
            autocomplete="off"
          />
        </div>
      </div>
      <div class="th-admin-divider-t mt-6 pt-6">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <p class="text-xs font-medium text-slate-400">{{ t('adminSystem.lblContactQrRows') }}</p>
          <button type="button" class="text-xs font-medium text-cyan-400/90 hover:underline" @click="s.addContactQrRow">
            {{ t('adminSystem.contactQrAddRow') }}
          </button>
        </div>
        <p v-if="s.contactPickQrIndex !== null" class="mt-2 text-[11px] text-amber-100/90">{{ t('adminSystem.contactPickActive') }}</p>
        <div v-if="!s.contactQRs.length" class="mt-2 text-[11px] text-slate-600">{{ t('adminSystem.contactQrEmpty') }}</div>
        <div v-for="(row, idx) in s.contactQRs" :key="idx" class="mt-4 th-admin-compare-tile">
          <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
            <span class="text-[11px] font-medium uppercase tracking-wide text-slate-500">{{ t('adminSystem.contactQrRow', { n: idx + 1 }) }}</span>
            <button type="button" class="text-[11px] text-rose-300/90 hover:underline" @click="s.removeContactQrRow(idx)">
              {{ t('adminSystem.contactQrRemove') }}
            </button>
          </div>
          <label class="mb-2 block text-xs text-slate-400">
            <span class="mb-1 block text-slate-500">{{ t('adminSystem.lblContactQrLabel') }}</span>
            <input v-model="row.label" type="text" class="th-input w-full text-sm" autocomplete="off" />
          </label>
          <label class="mb-2 block text-xs text-slate-400">
            <span class="mb-1 block text-slate-500">{{ t('adminSystem.lblContactQrUrl') }}</span>
            <input v-model="row.image_url" type="text" class="th-input w-full font-mono text-xs" autocomplete="off" />
          </label>
          <div class="mt-2 flex flex-wrap items-center gap-2">
            <label class="th-btn-secondary inline-flex cursor-pointer items-center px-3 py-1.5 text-[11px] font-medium select-none">
              <input
                type="file"
                accept="image/png,image/jpeg,image/gif,image/webp"
                class="sr-only"
                :disabled="s.uploadingContactRow !== null || s.saving"
                @change="s.uploadContactQr($event, idx)"
              />
              {{ s.uploadingContactRow === idx ? t('adminSystem.contactQrUploading') : t('adminSystem.contactQrUpload') }}
            </label>
            <button
              type="button"
              class="rounded border border-white/10 px-3 py-1.5 text-[11px] text-slate-300 hover:bg-white/[0.06]"
              :class="s.contactPickQrIndex === idx ? 'border-cyan-500/50 ring-1 ring-cyan-500/30' : ''"
              @click="s.startPickContactLibrary(idx)"
            >
              {{ t('adminSystem.contactQrPickLibrary') }}
            </button>
          </div>
          <div v-if="row.image_url.trim()" class="mt-2">
            <img :src="row.image_url.trim()" alt="" class="h-16 w-16 rounded border border-white/10 object-contain" />
          </div>
        </div>
      </div>
      <div class="mt-6">
        <p class="text-[11px] font-medium text-slate-400">{{ t('adminSystem.contactLibraryForQr') }}</p>
        <p class="mt-0.5 text-[11px] text-slate-500">{{ t('adminSystem.contactLibraryForQrHint') }}</p>
        <div class="mt-2 flex max-w-xl flex-wrap gap-2">
          <button
            v-for="ico in s.brandIconsUploadedOnly"
            :key="ico.url"
            type="button"
            :title="ico.filename || ico.url"
            class="h-11 w-11 overflow-hidden rounded-md th-admin-compare-tile--row !p-0 transition-colors hover:border-cyan-500/40"
            @click="s.applyContactLibraryIcon(ico)"
          >
            <img :src="ico.url.trim()" alt="" class="h-full w-full object-contain" />
          </button>
        </div>
        <p v-if="!s.brandIconsUploadedOnly.length" class="mt-2 text-[11px] text-slate-600">{{ t('adminSystem.contactLibraryEmpty') }}</p>
      </div>
    </section>
  </div>
</template>
