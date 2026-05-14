<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import contactCardImg from '../assets/xunji-contact-card.png'

const { t } = useI18n()
</script>

<template>
  <div class="max-w-2xl">
    <h1 class="mb-2 text-2xl font-bold">{{ t('billingPage.title') }}</h1>
    <p class="mb-6 text-sm text-slate-500">{{ t('billingPage.desc') }}</p>

    <section
      class="th-billing-wecom mb-8 rounded-2xl border-2 border-cyan-500/40 bg-gradient-to-br from-cyan-950/45 via-slate-900/80 to-slate-950/90 p-6 shadow-[0_20px_50px_-28px_rgba(34,211,238,0.35)] ring-1 ring-cyan-400/15"
      aria-labelledby="billing-wecom-heading"
    >
      <div class="flex flex-col gap-1 border-b border-cyan-500/20 pb-4 sm:flex-row sm:items-start sm:justify-between sm:gap-4">
        <div>
          <h2 id="billing-wecom-heading" class="text-base font-semibold tracking-tight text-cyan-100">
            {{ t('billingPage.wecomTitle') }}
          </h2>
          <p class="mt-2 max-w-xl text-sm leading-relaxed text-slate-300">{{ t('billingPage.wecomDesc') }}</p>
        </div>
        <span
          class="shrink-0 self-start rounded-full border border-amber-400/35 bg-amber-950/50 px-2.5 py-1 text-[10px] font-semibold uppercase tracking-wider text-amber-100/95"
        >
          {{ t('billingPage.wecomBadge') }}
        </span>
      </div>
      <div class="mt-5 flex flex-col items-center gap-3 sm:items-start">
        <div class="rounded-xl border-2 border-white/10 bg-white/[0.04] p-3 shadow-inner ring-2 ring-cyan-500/25">
          <img
            :src="contactCardImg"
            :alt="t('home.contactCardAlt')"
            width="300"
            loading="eager"
            decoding="async"
            class="mx-auto block h-auto w-full max-w-[min(100%,300px)] rounded-lg"
          />
        </div>
        <p class="max-w-md text-center text-xs leading-relaxed text-slate-400 sm:text-left">
          {{ t('home.contactScanHint') }}
        </p>
      </div>
    </section>

    <div class="space-y-4 rounded-xl border border-slate-800 bg-slate-900/60 p-6">
      <div class="flex items-start gap-3">
        <span class="mt-0.5 text-cyan-400">①</span>
        <div>
          <p class="font-medium text-slate-200">{{ t('billingPage.redeemTitle') }}</p>
          <p class="mt-1 text-sm text-slate-400">{{ t('billingPage.redeemDesc') }}</p>
          <RouterLink to="/console/redeem" class="mt-2 inline-block text-sm text-cyan-400 hover:underline">{{ t('billingPage.redeemLink') }}</RouterLink>
        </div>
      </div>
      <div class="flex items-start gap-3">
        <span class="mt-0.5 text-cyan-400">②</span>
        <div>
          <p class="font-medium text-slate-200">{{ t('billingPage.corpTitle') }}</p>
          <p class="mt-1 text-sm text-slate-400">{{ t('billingPage.corpDesc') }}</p>
        </div>
      </div>
      <div class="flex items-start gap-3 border-t border-slate-800 pt-4">
        <span class="mt-0.5 text-amber-500">!</span>
        <p class="text-sm text-slate-500">{{ t('billingPage.note') }}</p>
      </div>
    </div>
  </div>
</template>
