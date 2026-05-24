<script setup lang="ts">
import { inject } from 'vue'
import { useI18n } from 'vue-i18n'
import Failover4xxCodePicker from '../components/admin/Failover4xxCodePicker.vue'
import { adminSiteSettingsKey } from '../composables/useAdminSiteSettings'

const { t } = useI18n()
const s = inject(adminSiteSettingsKey)!
</script>

<template>
  <div class="space-y-8">
    <section class="th-glass rounded-[var(--th-radius-lg)] p-5 md:p-6">
      <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secGatewayRouting') }}</h2>
      <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secGatewayRoutingHint') }}</p>
      <div class="mt-5 max-w-xl space-y-4">
        <div>
          <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
            t('adminSystem.lblRoutingStrategy')
          }}</label>
          <select v-model="s.form.routingStrategy" class="th-input max-w-md text-sm">
            <option value="weighted_random">{{ t('adminSystem.routingWeighted') }}</option>
            <option value="priority">{{ t('adminSystem.routingPriority') }}</option>
            <option value="fixed_order">{{ t('adminSystem.routingFixedOrder') }}</option>
          </select>
        </div>
        <label class="flex cursor-pointer items-start gap-3">
          <input v-model="s.form.routingFailoverEnabled" type="checkbox" class="mt-1" />
          <span class="block text-xs text-slate-300">
            {{ t('adminSystem.lblRoutingFailover') }}
            <span class="mt-0.5 block text-[11px] text-slate-500">{{ t('adminSystem.hintRoutingFailoverRecommend') }}</span>
          </span>
        </label>
        <p v-if="s.form.routingFailoverEnabled" class="text-[11px] leading-relaxed text-slate-500">
          {{ t('adminSystem.hintRoutingFailoverOn') }}
        </p>
        <p class="text-[11px] leading-relaxed text-slate-500">{{ t('adminSystem.hintRoutingFailoverOff') }}</p>
        <div v-if="s.form.routingFailoverEnabled" class="space-y-3 rounded-lg border border-slate-800 bg-slate-950/40 p-3">
          <div>
            <label class="mb-1.5 block text-xs font-medium text-slate-500">{{ t('adminSystem.lblRoutingFailoverMax') }}</label>
            <input v-model.number="s.form.routingFailoverMaxAttempts" type="number" min="1" max="5" class="th-input max-w-xs font-mono text-sm" />
          </div>
          <div>
            <label class="mb-1.5 block text-xs font-medium text-slate-500">{{ t('adminGroups.lblFailover4xxMode') }}</label>
            <select v-model="s.form.routingFailover4xxMode" class="th-input max-w-md text-sm">
              <option value="off">{{ t('adminGroups.failover4xxModeOff') }}</option>
              <option value="selective">{{ t('adminGroups.failover4xxModeSelective') }} ({{ t('adminGroups.failover4xxBadgeRecommended') }})</option>
              <option value="custom">{{ t('adminGroups.failover4xxModeCustom') }}</option>
              <option value="aggressive">{{ t('adminGroups.failover4xxModeAggressive') }} ({{ t('adminGroups.failover4xxBadgeCaution') }})</option>
            </select>
            <p class="mt-1 text-[11px] text-slate-500">{{ t(`adminGroups.hintFailover4xxMode${s.form.routingFailover4xxMode === 'selective' ? 'Selective' : s.form.routingFailover4xxMode === 'aggressive' ? 'Aggressive' : s.form.routingFailover4xxMode === 'custom' ? 'Custom' : 'Off'}`) }}</p>
            <Failover4xxCodePicker v-model="s.form.routingFailover4xxCodes" :mode="s.form.routingFailover4xxMode" />
          </div>
          <div class="grid gap-3 sm:grid-cols-2">
            <div>
              <label class="mb-1 block text-xs text-slate-500">{{ t('adminGroups.lblFailoverFirstTimeout') }}</label>
              <input v-model.number="s.form.routingFailoverFirstTimeout" type="number" min="5" max="120" class="th-input font-mono text-sm" />
            </div>
            <div>
              <label class="mb-1 block text-xs text-slate-500">{{ t('adminGroups.lblFailoverRetryAfter') }}</label>
              <input v-model.number="s.form.routingFailoverRetryAfter" type="number" min="1" max="300" class="th-input font-mono text-sm" />
            </div>
          </div>
          <details class="text-[11px] text-slate-500">
            <summary class="cursor-pointer text-slate-400">{{ t('adminGroups.failover4xxHelpTitle') }}</summary>
            <p class="mt-2">{{ t('adminGroups.failover4xxHelpSummary') }}</p>
          </details>
        </div>
        <label class="flex cursor-pointer items-start gap-3">
          <input v-model="s.form.routingStickyEnabled" type="checkbox" class="mt-1" />
          <span class="text-xs text-slate-300">{{ t('adminSystem.lblRoutingSticky') }}</span>
        </label>
        <label class="flex cursor-pointer items-start gap-3">
          <input v-model="s.form.routingAutoHealthFromProbe" type="checkbox" class="mt-1" />
          <span class="text-xs text-slate-300">{{ t('adminSystem.lblRoutingAutoHealth') }}</span>
        </label>
        <div v-if="s.form.routingAutoHealthFromProbe">
          <label class="mb-1.5 block text-xs font-medium text-slate-500">{{ t('adminSystem.lblRoutingProbeThreshold') }}</label>
          <input v-model.number="s.form.routingProbeFailThreshold" type="number" min="1" max="20" class="th-input max-w-xs font-mono text-sm" />
        </div>
      </div>
    </section>

    <section class="th-glass rounded-[var(--th-radius-lg)] p-5 md:p-6">
      <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secUpstreamProbe') }}</h2>
      <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secUpstreamProbeHint') }}</p>
      <p class="mt-2 text-[11px] leading-relaxed text-slate-500">{{ t('adminSystem.hintUpstreamProbeListDisclaimer') }}</p>
      <div class="mt-5 max-w-xl space-y-4">
        <label class="flex cursor-pointer items-start gap-3">
          <input v-model="s.form.upstreamProbeEnabled" type="checkbox" class="mt-1" />
          <span class="block text-xs font-medium text-slate-300">{{ t('adminSystem.lblUpstreamProbeEnabled') }}</span>
        </label>
        <div>
          <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
            t('adminSystem.lblUpstreamProbeInterval')
          }}</label>
          <input
            v-model.number="s.form.upstreamProbeInterval"
            type="number"
            min="15"
            max="1440"
            step="1"
            class="th-input max-w-xs font-mono text-sm"
            autocomplete="off"
          />
          <p class="mt-2 text-[11px] leading-relaxed text-slate-500">{{ t('adminSystem.hintUpstreamProbeInterval') }}</p>
        </div>
        <p
          v-if="s.upstreamProbeEnabledDbOverride || s.upstreamProbeIntervalDbOverride"
          class="text-[11px] leading-relaxed text-amber-100/80"
        >
          {{ t('adminSystem.hintUpstreamProbeDb') }}
        </p>
        <div>
          <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
            t('adminSystem.lblUpstreamCapabilityFailStaleHours')
          }}</label>
          <input
            v-model.number="s.form.upstreamCapabilityFailStaleHours"
            type="number"
            min="0"
            max="168"
            step="1"
            class="th-input max-w-xs font-mono text-sm"
            autocomplete="off"
          />
          <p class="mt-2 text-[11px] leading-relaxed text-slate-500">{{ t('adminSystem.hintUpstreamCapabilityFailStaleHours') }}</p>
        </div>
        <label class="flex cursor-pointer items-start gap-3">
          <input v-model="s.form.upstreamCapabilityProbeEnabled" type="checkbox" class="mt-1" />
          <span class="block text-xs font-medium text-slate-300">{{ t('adminSystem.lblUpstreamCapabilityProbeEnabled') }}</span>
        </label>
        <p class="text-[11px] leading-relaxed text-slate-500">{{ t('adminSystem.hintUpstreamCapabilityProbeEnabled') }}</p>
      </div>
    </section>

    <section class="th-glass rounded-[var(--th-radius-lg)] p-5 md:p-6">
      <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secOps') }}</h2>
      <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secOpsHint') }}</p>
      <div class="mt-4">
        <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{ t('adminSystem.lblOperator') }}</label>
        <input v-model="s.form.operatorDisplayName" type="text" class="th-input max-w-xl" autocomplete="off" />
      </div>
    </section>

    <section class="th-glass rounded-[var(--th-radius-lg)] p-5 md:p-6">
      <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secGatewayAdminAlert') }}</h2>
      <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secGatewayAdminAlertHint') }}</p>
      <div class="mt-5 max-w-2xl space-y-4">
        <label class="flex cursor-pointer items-start gap-3">
          <input v-model="s.form.gatewayAdminAlertEnabled" type="checkbox" class="mt-1" />
          <span class="block text-xs text-slate-300">{{ t('adminSystem.lblGatewayAdminAlertEnabled') }}</span>
        </label>
        <template v-if="s.form.gatewayAdminAlertEnabled">
          <div class="space-y-2 rounded-lg border border-slate-800 bg-slate-950/40 p-3">
            <p class="text-[11px] font-medium uppercase tracking-wide text-slate-500">{{ t('adminSystem.lblGatewayAlertTriggers') }}</p>
            <label class="flex cursor-pointer items-center gap-2 text-xs text-slate-300">
              <input v-model="s.form.gwAlertNotify5xx" type="checkbox" /> {{ t('adminSystem.gwAlertTrigger5xx') }}
            </label>
            <label class="flex cursor-pointer items-center gap-2 text-xs text-slate-300">
              <input v-model="s.form.gwAlertNotifyTransport" type="checkbox" /> {{ t('adminSystem.gwAlertTriggerTransport') }}
            </label>
            <label class="flex cursor-pointer items-center gap-2 text-xs text-slate-300">
              <input v-model="s.form.gwAlertNotifyFailover" type="checkbox" /> {{ t('adminSystem.gwAlertTriggerFailover') }}
            </label>
            <label class="flex cursor-pointer items-center gap-2 text-xs text-slate-300">
              <input v-model="s.form.gwAlertNotify4xx" type="checkbox" /> {{ t('adminSystem.gwAlertTrigger4xx') }}
            </label>
            <label class="flex cursor-pointer items-center gap-2 text-xs text-slate-300">
              <input v-model="s.form.gwAlertNotify402" type="checkbox" /> {{ t('adminSystem.gwAlertTrigger402') }}
            </label>
          </div>
          <div class="space-y-2 rounded-lg border border-slate-800 bg-slate-950/40 p-3">
            <p class="text-[11px] font-medium uppercase tracking-wide text-slate-500">{{ t('adminSystem.lblGatewayAlertChannels') }}</p>
            <label class="flex cursor-pointer items-center gap-2 text-xs text-slate-300">
              <input v-model="s.form.gwAlertChannelInApp" type="checkbox" /> {{ t('adminSystem.gwAlertChannelInApp') }}
            </label>
            <label class="flex cursor-pointer items-center gap-2 text-xs text-slate-300">
              <input v-model="s.form.gwAlertChannelEmail" type="checkbox" /> {{ t('adminSystem.gwAlertChannelEmail') }}
            </label>
            <label class="flex cursor-pointer items-center gap-2 text-xs text-slate-300">
              <input v-model="s.form.gwAlertChannelDingtalk" type="checkbox" /> {{ t('adminSystem.gwAlertChannelDingtalk') }}
            </label>
            <label class="flex cursor-pointer items-center gap-2 text-xs text-slate-300">
              <input v-model="s.form.gwAlertChannelFeishu" type="checkbox" /> {{ t('adminSystem.gwAlertChannelFeishu') }}
            </label>
            <label class="flex cursor-pointer items-center gap-2 text-xs text-slate-300">
              <input v-model="s.form.gwAlertChannelWecom" type="checkbox" /> {{ t('adminSystem.gwAlertChannelWecom') }}
            </label>
            <label class="flex cursor-pointer items-center gap-2 text-xs text-slate-300">
              <input v-model="s.form.gwAlertChannelGenericWebhook" type="checkbox" /> {{ t('adminSystem.gwAlertChannelGeneric') }}
            </label>
          </div>
          <div v-if="s.form.gwAlertChannelEmail" class="space-y-2">
            <label class="mb-1 block text-xs text-slate-500">{{ t('adminSystem.lblGatewayAlertEmails') }}</label>
            <textarea v-model="s.form.gwAlertEmailsText" rows="3" class="th-input w-full font-mono text-sm" :placeholder="t('adminSystem.phGatewayAlertEmails')" />
          </div>
          <div v-if="s.form.gwAlertChannelDingtalk" class="space-y-2">
            <label class="mb-1 block text-xs text-slate-500">{{ t('adminSystem.lblGatewayAlertDingtalk') }}</label>
            <input v-model="s.form.gwAlertDingtalkWebhook" type="url" class="th-input w-full font-mono text-sm" autocomplete="off" />
            <input v-model="s.gwAlertSecretDing" type="password" class="th-input w-full font-mono text-sm" :placeholder="t('adminSystem.phAlertSignSecret')" autocomplete="new-password" />
            <label v-if="s.gwAlertSignConfigured?.dingtalk_sign_configured" class="flex items-center gap-2 text-[11px] text-slate-500">
              <input v-model="s.gwAlertClearDingSign" type="checkbox" /> {{ t('adminSystem.clearAlertSign') }}
            </label>
          </div>
          <div v-if="s.form.gwAlertChannelFeishu" class="space-y-2">
            <label class="mb-1 block text-xs text-slate-500">{{ t('adminSystem.lblGatewayAlertFeishu') }}</label>
            <input v-model="s.form.gwAlertFeishuWebhook" type="url" class="th-input w-full font-mono text-sm" autocomplete="off" />
            <input v-model="s.gwAlertSecretFeishu" type="password" class="th-input w-full font-mono text-sm" :placeholder="t('adminSystem.phAlertSignSecret')" autocomplete="new-password" />
            <label v-if="s.gwAlertSignConfigured?.feishu_sign_configured" class="flex items-center gap-2 text-[11px] text-slate-500">
              <input v-model="s.gwAlertClearFeishuSign" type="checkbox" /> {{ t('adminSystem.clearAlertSign') }}
            </label>
          </div>
          <div v-if="s.form.gwAlertChannelWecom" class="space-y-2">
            <label class="mb-1 block text-xs text-slate-500">{{ t('adminSystem.lblGatewayAlertWecom') }}</label>
            <input v-model="s.form.gwAlertWecomWebhook" type="url" class="th-input w-full font-mono text-sm" autocomplete="off" />
            <input v-model="s.gwAlertSecretWecom" type="password" class="th-input w-full font-mono text-sm" :placeholder="t('adminSystem.phAlertSignSecret')" autocomplete="new-password" />
            <label v-if="s.gwAlertSignConfigured?.wecom_sign_configured" class="flex items-center gap-2 text-[11px] text-slate-500">
              <input v-model="s.gwAlertClearWecomSign" type="checkbox" /> {{ t('adminSystem.clearAlertSign') }}
            </label>
          </div>
          <div v-if="s.form.gwAlertChannelGenericWebhook" class="space-y-2">
            <label class="mb-1 block text-xs text-slate-500">{{ t('adminSystem.lblGatewayAlertGeneric') }}</label>
            <input v-model="s.form.gwAlertGenericWebhookUrl" type="url" class="th-input w-full font-mono text-sm" autocomplete="off" />
            <input v-model="s.gwAlertSecretGeneric" type="password" class="th-input w-full font-mono text-sm" :placeholder="t('adminSystem.phAlertSignSecret')" autocomplete="new-password" />
            <label v-if="s.gwAlertSignConfigured?.generic_webhook_sign_configured" class="flex items-center gap-2 text-[11px] text-slate-500">
              <input v-model="s.gwAlertClearGenericSign" type="checkbox" /> {{ t('adminSystem.clearAlertSign') }}
            </label>
          </div>
          <div class="grid gap-3 sm:grid-cols-2">
            <div>
              <label class="mb-1 block text-xs text-slate-500">{{ t('adminSystem.lblGatewayAlertCooldown') }}</label>
              <input v-model.number="s.form.gwAlertCooldownSec" type="number" min="60" max="3600" class="th-input font-mono text-sm" />
            </div>
            <div>
              <label class="mb-1 block text-xs text-slate-500">{{ t('adminSystem.lblGatewayAlertMinInterval') }}</label>
              <input v-model.number="s.form.gwAlertMinIntervalSec" type="number" min="10" max="600" class="th-input font-mono text-sm" />
            </div>
          </div>
          <p class="text-[11px] leading-relaxed text-slate-500">{{ t('adminSystem.hintGatewayAdminAlertPerf') }}</p>
        </template>
      </div>
    </section>

    <section class="rounded-lg border border-dashed border-white/[0.1] bg-black/[0.06] p-4">
      <p class="text-xs text-slate-500">{{ t('adminSystem.moreHint') }}</p>
    </section>
  </div>
</template>
