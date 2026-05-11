<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'

const { t } = useI18n()
</script>

<template>
  <div class="max-w-2xl">
    <h1 class="mb-2 text-2xl font-bold">{{ t('billingPage.title') }}</h1>
    <p class="mb-6 text-sm text-slate-500">{{ t('billingPage.desc') }}</p>
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
