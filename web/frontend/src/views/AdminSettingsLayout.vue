<script setup lang="ts">
import { onMounted, provide } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { adminSiteSettingsKey, createAdminSiteSettings } from '../composables/useAdminSiteSettings'

const ctx = createAdminSiteSettings()
provide(adminSiteSettingsKey, ctx)

const { t } = useI18n()
const route = useRoute()

onMounted(() => {
  void ctx.refresh()
})

const settingTabs = [
  { name: 'admin-settings-brand' as const, labelKey: 'adminSystem.tabBrand' as const },
  { name: 'admin-settings-public' as const, labelKey: 'adminSystem.tabPublic' as const },
  { name: 'admin-settings-signup' as const, labelKey: 'adminSystem.tabSignup' as const },
  { name: 'admin-settings-no-bill' as const, labelKey: 'adminSystem.tabNoBill' as const },
  { name: 'admin-settings-security' as const, labelKey: 'adminSystem.tabSecurity' as const },
  { name: 'admin-settings-gateway-ops' as const, labelKey: 'adminSystem.tabGatewayOps' as const },
]
</script>

<template>
  <div class="th-admin-settings-root">
    <h1 class="th-admin-settings-title text-xl font-semibold tracking-tight">{{ t('adminSystem.title') }}</h1>
    <p class="mt-1 text-sm text-slate-500">{{ t('adminSystem.layoutIntro') }}</p>

    <nav
      class="th-admin-divider-b mt-6 flex flex-wrap gap-1.5 pb-4 sm:gap-2"
      aria-label="System settings sections"
    >
      <RouterLink
        v-for="tab in settingTabs"
        :key="tab.name"
        :to="{ name: tab.name }"
        class="th-admin-settings-tab rounded-lg px-2 py-1.5 text-xs font-medium sm:px-3 sm:py-2 sm:text-sm"
        :class="{ 'th-admin-settings-tab--active': route.name === tab.name }"
      >
        {{ t(tab.labelKey) }}
      </RouterLink>
    </nav>

    <p v-if="ctx.err" class="th-admin-flash--error mt-4 rounded-lg border px-3 py-2 text-sm">{{ ctx.err }}</p>
    <p v-if="ctx.msg" class="th-admin-flash--success mt-4 rounded-lg border px-3 py-2 text-sm">{{ ctx.msg }}</p>

    <div v-if="ctx.busy" class="mt-8 text-sm text-slate-500">{{ t('common.loading') }}</div>

    <div v-else class="mt-6">
      <RouterView />
    </div>

    <div v-if="!ctx.busy" class="th-admin-divider-t mt-10 flex flex-wrap gap-3 pt-8">
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
</template>
