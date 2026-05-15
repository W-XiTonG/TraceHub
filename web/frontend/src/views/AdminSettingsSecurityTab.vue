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
      <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secSecurity') }}</h2>
      <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secSecurityHint') }}</p>
      <label class="mt-4 flex cursor-pointer items-center gap-2.5 text-sm text-slate-200 select-none">
        <input v-model="s.form.loginCaptcha" type="checkbox" class="h-4 w-4 shrink-0 rounded border-slate-600 text-cyan-500" />
        <span>{{ t('adminSystem.chkLoginCaptcha') }}</span>
      </label>
      <div class="th-admin-divider-t mt-6 max-w-2xl pt-6">
        <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
          t('adminSystem.lblGatewayIpBlacklist')
        }}</label>
        <p class="mb-3 text-[11px] leading-relaxed text-slate-500">{{ t('adminSystem.hintGatewayIpBlacklist') }}</p>
        <div class="th-admin-table-shell">
          <table class="w-full border-collapse text-left text-xs">
            <thead class="th-admin-table-head">
              <tr>
                <th class="px-3 py-2 font-medium text-slate-500">{{ t('adminSystem.thGatewayIp') }}</th>
                <th class="w-28 px-2 py-2 text-center font-medium text-slate-500">{{ t('adminCommon.delete') }}</th>
              </tr>
            </thead>
            <tbody class="th-admin-table-body">
              <tr v-if="s.gatewayIpBlacklistRows.length === 0">
                <td colspan="2" class="px-3 py-4 text-center text-[11px] text-slate-500">
                  {{ t('adminSystem.gatewayIpNoRows') }}
                </td>
              </tr>
              <tr v-for="(row, idx) in s.gatewayIpBlacklistRows" :key="idx">
                <td class="px-2 py-2">
                  <input
                    v-model="row.value"
                    type="text"
                    class="th-input h-9 w-full min-w-[12rem] py-1 font-mono text-xs leading-normal"
                    :placeholder="t('adminSystem.phGatewayIpBlacklist')"
                    autocomplete="off"
                    spellcheck="false"
                  />
                </td>
                <td class="px-2 py-2 text-center align-middle">
                  <button
                    type="button"
                    class="th-btn-secondary px-2 py-1 text-[11px] font-medium"
                    :disabled="s.saving"
                    @click="s.removeGatewayIpBlacklistRow(idx)"
                  >
                    {{ t('adminCommon.delete') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="mt-3 flex flex-wrap gap-2">
          <button type="button" class="th-btn-secondary px-3 py-1.5 text-xs" :disabled="s.saving" @click="s.addGatewayIpBlacklistRow">
            {{ t('adminGroups.addRow') }}
          </button>
        </div>
        <p v-if="s.gatewayIpBlacklistDbOverride" class="mt-3 text-[11px] leading-relaxed text-amber-100/80">
          {{ t('adminSystem.hintGatewayIpBlacklistDb') }}
        </p>
      </div>
    </section>
  </div>
</template>
