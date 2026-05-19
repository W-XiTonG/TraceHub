<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { useSiteMetaStore } from '../../stores/siteMeta'

const { t } = useI18n()
const siteMeta = useSiteMetaStore()

const emit = defineEmits<{
  navigate: []
}>()

function onNavClick() {
  emit('navigate')
}
</script>

<template>
  <RouterLink
    to="/"
    :title="t('layout.adminBackTitle')"
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
      <p class="th-sidebar-brand th-admin-sidebar-brand text-sm font-semibold">{{ t('layout.adminTitle') }}</p>
      <p class="text-[10px] uppercase tracking-[0.18em] text-slate-600">{{ t('layout.adminTag') }}</p>
    </div>
  </RouterLink>
  <nav class="flex flex-1 flex-col gap-0.5 text-sm" :aria-label="t('layout.mobileNavAria')">
    <RouterLink :to="{ name: 'admin-dash' }" class="th-nav-link th-nav-link--amber" @click="onNavClick">{{ t('layout.adminNavOverview') }}</RouterLink>
    <RouterLink to="/admin/users" class="th-nav-link th-nav-link--amber" @click="onNavClick">{{ t('layout.adminNavUsers') }}</RouterLink>
    <RouterLink to="/admin/channels" class="th-nav-link th-nav-link--amber" @click="onNavClick">{{ t('layout.adminNavChannels') }}</RouterLink>
    <RouterLink :to="{ name: 'admin-groups' }" class="th-nav-link th-nav-link--amber" @click="onNavClick">{{ t('layout.adminNavGroups') }}</RouterLink>
    <RouterLink to="/admin/logs" class="th-nav-link th-nav-link--amber" @click="onNavClick">{{ t('layout.adminNavLogs') }}</RouterLink>
    <RouterLink to="/admin/balance-ledger" class="th-nav-link th-nav-link--amber" @click="onNavClick">{{ t('layout.adminNavBalanceLedger') }}</RouterLink>
    <RouterLink to="/admin/model-market" class="th-nav-link th-nav-link--amber" @click="onNavClick">{{ t('layout.adminNavMarket') }}</RouterLink>
    <RouterLink to="/admin/announcements" class="th-nav-link th-nav-link--amber" @click="onNavClick">{{ t('layout.adminNavAnnouncements') }}</RouterLink>
    <RouterLink to="/admin/redeem-codes" class="th-nav-link th-nav-link--amber" @click="onNavClick">{{ t('layout.adminNavRedeem') }}</RouterLink>
    <RouterLink to="/admin/payment/config" class="th-nav-link th-nav-link--amber" @click="onNavClick">{{ t('layout.adminNavPayment') }}</RouterLink>
    <RouterLink v-if="siteMeta.onlinePaymentEnabled" to="/admin/payment/orders" class="th-nav-link th-nav-link--amber" @click="onNavClick">
      {{ t('layout.adminNavPaymentOrders') }}
    </RouterLink>
    <RouterLink v-if="siteMeta.onlinePaymentEnabled" to="/admin/payment/packages" class="th-nav-link th-nav-link--amber" @click="onNavClick">
      {{ t('layout.adminNavPaymentPackages') }}
    </RouterLink>
    <RouterLink to="/admin/redeem-ledger" class="th-nav-link th-nav-link--amber" @click="onNavClick">{{ t('layout.adminNavRedeemLedger') }}</RouterLink>
    <RouterLink to="/admin/feedbacks" class="th-nav-link th-nav-link--amber" @click="onNavClick">{{ t('layout.adminNavFeedbacks') }}</RouterLink>
    <RouterLink :to="{ name: 'admin-billing-global-pricing' }" class="th-nav-link th-nav-link--amber" @click="onNavClick">{{ t('layout.adminNavBilling') }}</RouterLink>
    <RouterLink :to="{ name: 'admin-settings-brand' }" class="th-nav-link th-nav-link--amber" @click="onNavClick">{{ t('layout.adminNavSystem') }}</RouterLink>
    <RouterLink to="/docs" class="th-nav-link th-nav-link--amber border-t border-white/[0.06] pt-3 mt-2" @click="onNavClick">{{ t('layout.navDocs') }}</RouterLink>
  </nav>
  <div class="mt-4 flex flex-col gap-2 border-t border-white/[0.06] pt-4">
    <RouterLink
      to="/console"
      :title="t('layout.adminConsoleTitle')"
      class="th-admin-user-console-link flex items-center gap-2 px-3 py-2 text-xs font-medium"
      @click="onNavClick"
    >
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-3.5 w-3.5 shrink-0 opacity-90" aria-hidden="true">
        <path d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4zM3 10a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H4a1 1 0 01-1-1v-6zM14 9a1 1 0 00-1 1v6a1 1 0 001 1h2a1 1 0 001-1v-6a1 1 0 00-1-1h-2z" />
      </svg>
      {{ t('layout.adminConsoleShort') }}
    </RouterLink>
    <RouterLink
      to="/"
      :title="t('layout.adminBackTitle')"
      class="flex items-center gap-2 rounded-lg px-3 py-2 text-xs font-medium text-slate-500 transition hover:bg-white/[0.06] hover:text-slate-300"
      @click="onNavClick"
    >
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-3.5 w-3.5 shrink-0 opacity-80" aria-hidden="true">
        <path d="M10.707 2.293a1 1 0 00-1.414 0l-7 7a1 1 0 101.414 1.414L4 10.414V17a1 1 0 001 1h2a1 1 0 001-1v-2a1 1 0 011-1h2a1 1 0 011 1v2a1 1 0 001 1h2a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7z" />
      </svg>
      {{ t('nav.siteHome') }}
    </RouterLink>
  </div>
</template>
