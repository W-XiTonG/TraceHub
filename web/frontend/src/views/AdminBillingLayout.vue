<script setup lang="ts">
import { computed, onMounted, provide } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import AdminActionBanner from '../components/admin/AdminActionBanner.vue'
import { adminSiteSettingsKey, createAdminSiteSettings } from '../composables/useAdminSiteSettings'

const ctx = createAdminSiteSettings()
provide(adminSiteSettingsKey, ctx)

const { t } = useI18n()
const route = useRoute()

const hideGlobalSaveBar = computed(
  () =>
    route.name === 'admin-billing-global-pricing' ||
    route.name === 'admin-billing-pricing-conversion' ||
    route.name === 'admin-billing-pricing-sync',
)

const billingTabs = [
  { name: 'admin-billing-global-pricing' as const, labelKey: 'adminBilling.tabGlobalPricing' as const },
  { name: 'admin-billing-pricing-conversion' as const, labelKey: 'adminBilling.tabConversion' as const },
  { name: 'admin-billing-no-bill' as const, labelKey: 'adminBilling.tabNoBill' as const },
  { name: 'admin-billing-pricing-sync' as const, labelKey: 'adminBilling.tabPricingSync' as const },
]

onMounted(() => {
  void ctx.refresh()
})
</script>

<template>
  <div class="th-admin-settings-root">
    <h1 class="th-admin-settings-title text-xl font-semibold tracking-tight">{{ t('adminBilling.title') }}</h1>
    <p class="mt-1 text-sm text-slate-500">{{ t('adminBilling.layoutIntro') }}</p>

    <nav
      class="th-admin-divider-b mt-6 flex flex-wrap gap-1.5 pb-4 sm:gap-2"
      aria-label="Billing admin sections"
    >
      <RouterLink
        v-for="tab in billingTabs"
        :key="tab.name"
        :to="{ name: tab.name }"
        class="th-admin-settings-tab rounded-lg px-2 py-1.5 text-xs font-medium sm:px-3 sm:py-2 sm:text-sm"
        :class="{ 'th-admin-settings-tab--active': route.name === tab.name }"
      >
        {{ t(tab.labelKey) }}
      </RouterLink>
    </nav>

    <ThLoadingState v-if="ctx.busyUi" class="mt-8" />

    <div v-else class="mt-6">
      <RouterView />
    </div>

    <div v-if="!hideGlobalSaveBar && !ctx.busy" class="th-admin-divider-t mt-10 space-y-4 pt-8">
      <AdminActionBanner
        :message="ctx.err || ctx.msg"
        :is-error="!!ctx.err"
        :loading="ctx.saving || ctx.resettingDefaults"
      />
      <div class="flex flex-wrap gap-3">
        <button type="button" class="th-btn-primary px-5 py-2.5 text-sm" :disabled="ctx.saving" @click="ctx.save">
          {{ ctx.saving ? t('common.submitting') : t('adminCommon.save') }}
        </button>
        <button type="button" class="th-btn-secondary px-5 py-2.5 text-sm" :disabled="ctx.saving || ctx.busy" @click="ctx.refresh">
          {{ t('adminCommon.refresh') }}
        </button>
        <button
          type="button"
          class="th-btn-secondary border-amber-500/25 px-5 py-2.5 text-sm text-amber-100/90 hover:border-amber-500/40"
          :disabled="
            ctx.saving ||
            ctx.busy ||
            ctx.resettingDefaults ||
            ctx.uploadingIcon ||
            ctx.deletingBrandFilename !== null ||
            ctx.contactPickQrIndex !== null
          "
          @click="ctx.resetDefaults"
        >
          {{ ctx.resettingDefaults ? t('common.loading') : t('adminSystem.resetDefaults') }}
        </button>
      </div>
    </div>
  </div>
</template>
