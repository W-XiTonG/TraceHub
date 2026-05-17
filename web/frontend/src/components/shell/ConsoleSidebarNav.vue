<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { useAuthStore } from '../../stores/auth'
import { useSiteMetaStore } from '../../stores/siteMeta'

const { t } = useI18n()
const auth = useAuthStore()
const siteMeta = useSiteMetaStore()

const emit = defineEmits<{
  navigate: []
}>()

const canAccessAdmin = computed(() => ['admin', 'superadmin'].includes(auth.role))

function onNavClick() {
  emit('navigate')
}
</script>

<template>
  <RouterLink
    to="/"
    :title="t('layout.consoleBackTitle')"
    class="group mb-8 flex items-center gap-3 rounded-xl px-2 py-1.5 outline-none transition hover:bg-white/[0.05]"
    @click="onNavClick"
  >
    <div class="th-logo-mark th-logo-mark--md th-logo-mark--image shrink-0 transition group-hover:brightness-105">
      <img
        :key="siteMeta.displayIconUrl"
        :src="siteMeta.displayIconUrl"
        :alt="siteMeta.displayName"
        width="40"
        height="40"
        decoding="async"
      />
    </div>
    <div class="min-w-0">
      <p class="th-sidebar-brand text-sm font-semibold text-slate-100">
        <span>{{ siteMeta.displayName }}</span>
        <span class="font-normal text-slate-500"> · </span>
        <span class="text-slate-200">{{ t('layout.consoleTitle') }}</span>
      </p>
      <p class="text-[10px] uppercase tracking-[0.18em] text-slate-600">{{ t('layout.consoleUserTag') }}</p>
    </div>
  </RouterLink>
  <nav class="flex flex-1 flex-col gap-0.5 text-sm" :aria-label="t('layout.mobileNavAria')">
    <RouterLink :to="{ name: 'console' }" class="th-nav-link" @click="onNavClick">{{ t('layout.navOverview') }}</RouterLink>
    <RouterLink :to="{ name: 'console-models' }" class="th-nav-link" @click="onNavClick">{{ t('layout.navModels') }}</RouterLink>
    <RouterLink to="/console/keys" class="th-nav-link" @click="onNavClick">{{ t('layout.navKeys') }}</RouterLink>
    <RouterLink to="/console/logs" class="th-nav-link" @click="onNavClick">{{ t('layout.navLogs') }}</RouterLink>
    <RouterLink to="/console/balance-ledger" class="th-nav-link" @click="onNavClick">{{ t('layout.navLedger') }}</RouterLink>
    <RouterLink to="/console/group" class="th-nav-link" @click="onNavClick">{{ t('layout.navGroup') }}</RouterLink>
    <RouterLink to="/console/billing" class="th-nav-link" @click="onNavClick">{{ t('layout.navBilling') }}</RouterLink>
    <RouterLink v-if="siteMeta.onlinePaymentEnabled" to="/console/payment-orders" class="th-nav-link" @click="onNavClick">
      {{ t('layout.navPaymentOrders') }}
    </RouterLink>
    <RouterLink to="/console/redeem" class="th-nav-link" @click="onNavClick">{{ t('layout.navRedeem') }}</RouterLink>
    <RouterLink to="/console/settings" class="th-nav-link" @click="onNavClick">{{ t('layout.navSettings') }}</RouterLink>
    <RouterLink to="/docs" class="th-nav-link" @click="onNavClick">{{ t('layout.navDocs') }}</RouterLink>
    <div class="mt-2 flex flex-col gap-0.5 border-t border-white/[0.06] pt-3">
      <RouterLink :to="{ name: 'console-playground' }" class="th-nav-link" @click="onNavClick">{{ t('layout.navPlayground') }}</RouterLink>
      <RouterLink v-if="canAccessAdmin" to="/admin" class="th-nav-link" @click="onNavClick">{{ t('layout.navAdmin') }}</RouterLink>
    </div>
  </nav>
  <RouterLink
    to="/"
    class="mt-4 flex items-center gap-2 rounded-lg px-3 py-2 text-xs font-medium text-slate-500 transition hover:bg-white/[0.06] hover:text-slate-300"
    @click="onNavClick"
  >
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-3.5 w-3.5 shrink-0 opacity-80" aria-hidden="true">
      <path d="M10.707 2.293a1 1 0 00-1.414 0l-7 7a1 1 0 101.414 1.414L4 10.414V17a1 1 0 001 1h2a1 1 0 001-1v-2a1 1 0 011-1h2a1 1 0 011 1v2a1 1 0 001 1h2a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7z" />
    </svg>
    {{ t('nav.siteHome') }}
  </RouterLink>
</template>
