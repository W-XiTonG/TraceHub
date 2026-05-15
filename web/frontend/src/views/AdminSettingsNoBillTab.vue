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
      <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secBilling') }}</h2>
      <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secBillingHint') }}</p>
      <div class="mt-4 max-w-2xl">
        <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
          t('adminSystem.lblNoBillModels')
        }}</label>
        <textarea
          v-model="s.form.noBillModelsText"
          rows="5"
          class="th-input w-full resize-y font-mono text-xs leading-relaxed"
          :placeholder="t('adminSystem.phNoBillModels')"
          autocomplete="off"
        />
        <p class="mt-2 text-[11px] leading-relaxed text-slate-500">{{ t('adminSystem.hintNoBillModels') }}</p>
        <dl v-if="s.noBillModelsYaml.trim()" class="mt-2 th-admin-compare-tile">
          <dt class="text-[10px] uppercase tracking-wide text-slate-500">{{ t('adminSystem.lblNoBillModelsYaml') }}</dt>
          <dd class="m-0 mt-1 whitespace-pre-wrap font-mono text-[11px] leading-relaxed">{{ s.noBillModelsYaml }}</dd>
        </dl>
        <p v-if="s.noBillModelsDbOverride" class="mt-2 text-[11px] leading-relaxed text-amber-100/80">
          {{ t('adminSystem.hintNoBillModelsDb') }}
        </p>
      </div>
    </section>
  </div>
</template>
