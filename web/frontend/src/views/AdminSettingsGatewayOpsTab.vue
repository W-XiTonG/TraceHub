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
          <span class="text-xs text-slate-300">{{ t('adminSystem.lblRoutingFailover') }}</span>
        </label>
        <div v-if="s.form.routingFailoverEnabled">
          <label class="mb-1.5 block text-xs font-medium text-slate-500">{{ t('adminSystem.lblRoutingFailoverMax') }}</label>
          <input v-model.number="s.form.routingFailoverMaxAttempts" type="number" min="1" max="5" class="th-input max-w-xs font-mono text-sm" />
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

    <section class="rounded-lg border border-dashed border-white/[0.1] bg-black/[0.06] p-4">
      <p class="text-xs text-slate-500">{{ t('adminSystem.moreHint') }}</p>
    </section>
  </div>
</template>
