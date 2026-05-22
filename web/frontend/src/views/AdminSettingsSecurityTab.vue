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
      <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secHTTPS') }}</h2>
      <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secHTTPSHint') }}</p>
      <div class="mt-5 max-w-2xl space-y-4">
        <label class="flex cursor-pointer items-start gap-3">
          <input v-model="s.form.httpsForceRedirect" type="checkbox" class="mt-1" />
          <span class="block text-xs text-slate-300">
            {{ t('adminSystem.chkHTTPSForceRedirect') }}
            <span class="mt-0.5 block text-[11px] text-slate-500">{{ t('adminSystem.hintHTTPSForceRedirect') }}</span>
          </span>
        </label>
        <label class="flex cursor-pointer items-start gap-3">
          <input v-model="s.form.httpsHstsEnabled" type="checkbox" class="mt-1" />
          <span class="block text-xs text-slate-300">{{ t('adminSystem.chkHTTPSHSTS') }}</span>
        </label>
        <div v-if="s.form.httpsHstsEnabled">
          <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
            t('adminSystem.lblHTTPSHSTSMaxAge')
          }}</label>
          <input
            v-model.number="s.form.httpsHstsMaxAgeSeconds"
            type="number"
            min="0"
            max="63072000"
            class="th-input max-w-xs text-sm"
          />
          <p class="mt-1 text-[11px] text-slate-500">{{ t('adminSystem.hintHTTPSHSTSMaxAge') }}</p>
        </div>
        <div class="th-admin-divider-t pt-5">
          <p
            v-if="!s.tlsMaterialReady"
            class="mb-3 rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-[11px] leading-relaxed text-amber-100/90"
          >
            {{ t('adminSystem.hintTLSNeedMaterial') }}
          </p>
          <label class="flex cursor-pointer items-start gap-3" :class="{ 'opacity-60': !s.tlsMaterialReady && !s.form.tlsEnabled }">
            <input
              type="checkbox"
              class="mt-1"
              :checked="s.form.tlsEnabled"
              :disabled="s.saving || (!s.tlsMaterialReady && !s.form.tlsEnabled)"
              @change="s.onTlsEnabledChange"
            />
            <span class="block text-xs text-slate-300">
              {{ t('adminSystem.chkTLSEnabled') }}
              <span class="mt-0.5 block text-[11px] text-amber-100/80">{{ t('adminSystem.hintTLSERestart') }}</span>
              <span v-if="!s.tlsMaterialReady" class="mt-0.5 block text-[11px] text-amber-200/90">{{
                t('adminSystem.hintTLSEnableBlocked')
              }}</span>
            </span>
          </label>
          <dl class="mt-4 space-y-2 text-xs text-slate-400">
            <div class="flex flex-wrap gap-x-2">
              <dt class="text-slate-500">{{ t('adminSystem.lblTLSAddr') }}</dt>
              <dd class="font-mono text-slate-300">{{ s.tlsAddr || '—' }}</dd>
            </div>
            <div class="flex flex-wrap gap-x-2">
              <dt class="text-slate-500">{{ t('adminSystem.lblTLSCertFile') }}</dt>
              <dd class="font-mono text-slate-300 break-all">{{ s.tlsCertFile || '—' }}</dd>
              <span v-if="s.tlsCertFile && !s.tlsCertPresent" class="text-amber-200/90">{{ t('adminSystem.tlsCertMissing') }}</span>
            </div>
            <div class="flex flex-wrap gap-x-2">
              <dt class="text-slate-500">{{ t('adminSystem.lblTLSKeyFile') }}</dt>
              <dd class="font-mono text-slate-300 break-all">{{ s.tlsKeyFile || '—' }}</dd>
              <span v-if="s.tlsKeyFile && !s.tlsKeyPresent" class="text-amber-200/90">{{ t('adminSystem.tlsKeyMissing') }}</span>
            </div>
          </dl>
          <p v-if="s.tlsCertDbOverride || s.tlsKeyDbOverride" class="mt-2 text-[11px] text-amber-100/80">
            {{ t('adminSystem.hintTLSUploadDb') }}
          </p>
          <div class="mt-4 flex flex-wrap gap-3">
            <label class="th-btn-secondary cursor-pointer px-3 py-2 text-xs">
              <input
                type="file"
                class="hidden"
                accept=".pem,.crt,.cer,application/x-pem-file,text/plain"
                :disabled="s.uploadingTlsCert || s.saving"
                @change="s.uploadTlsCert"
              />
              {{ s.uploadingTlsCert ? t('adminSystem.tlsCertUploading') : t('adminSystem.btnUploadTLSCert') }}
            </label>
            <label class="th-btn-secondary cursor-pointer px-3 py-2 text-xs">
              <input
                type="file"
                class="hidden"
                accept=".pem,.key,application/x-pem-file,text/plain"
                :disabled="s.uploadingTlsKey || s.saving"
                @change="s.uploadTlsKey"
              />
              {{ s.uploadingTlsKey ? t('adminSystem.tlsKeyUploading') : t('adminSystem.btnUploadTLSKey') }}
            </label>
          </div>
          <p class="mt-2 text-[11px] leading-relaxed text-slate-500">{{ t('adminSystem.hintTLSUpload') }}</p>
        </div>
      </div>
    </section>

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
