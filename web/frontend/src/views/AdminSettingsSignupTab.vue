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
      <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secNewUser') }}</h2>
      <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secNewUserHint') }}</p>
      <div class="mt-4 max-w-md">
        <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
          t('adminSystem.lblDefaultNewUserBalance')
        }}</label>
        <input
          v-model.number="s.form.defaultNewUserBalance"
          type="number"
          min="0"
          step="1"
          class="th-input w-full font-mono text-sm"
          autocomplete="off"
        />
        <p class="mt-2 text-[11px] leading-relaxed text-slate-500">
          {{ t('adminSystem.hintDefaultNewUserYaml', { n: s.defaultNewUserBalanceYaml }) }}
        </p>
        <p v-if="s.defaultNewUserBalanceDbOverride" class="mt-1 text-[11px] leading-relaxed text-amber-100/80">
          {{ t('adminSystem.hintDefaultNewUserDb') }}
        </p>
      </div>
      <div class="th-admin-divider-t mt-6 max-w-2xl pt-6">
        <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
          t('adminSystem.lblRegisterPromo')
        }}</label>
        <textarea
          v-model="s.form.registerPromoText"
          rows="4"
          class="th-input w-full resize-y font-sans text-sm leading-relaxed"
          :placeholder="t('adminSystem.phRegisterPromo')"
          autocomplete="off"
        />
        <p class="mt-2 text-[11px] leading-relaxed text-slate-500">{{ t('adminSystem.hintRegisterPromoYaml') }}</p>
        <dl v-if="s.registerPromoTextYaml.trim()" class="mt-1 th-admin-compare-tile">
          <dd class="m-0 whitespace-pre-wrap text-[11px] leading-relaxed">{{ s.registerPromoTextYaml }}</dd>
        </dl>
        <p v-if="s.registerPromoTextDbOverride" class="mt-2 text-[11px] leading-relaxed text-amber-100/80">
          {{ t('adminSystem.hintRegisterPromoDb') }}
        </p>
      </div>
    </section>
  </div>
</template>
