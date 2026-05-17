<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { useAuthStore } from '../../stores/auth'
import { useSiteMetaStore } from '../../stores/siteMeta'
import { siteNavDrawerOpen } from '../../composables/shellNavState'
import RegisterPromoDropdown from '../RegisterPromoDropdown.vue'
import UserAccountMenu from '../UserAccountMenu.vue'

const { t } = useI18n()
const auth = useAuthStore()
const siteMeta = useSiteMetaStore()

const open = ref(false)

watch(open, (v) => {
  siteNavDrawerOpen.value = v
  if (typeof document === 'undefined') return
  document.body.style.overflow = v ? 'hidden' : ''
})

function close() {
  open.value = false
}

function openDrawer() {
  open.value = true
}

function onNavClick() {
  close()
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && open.value) close()
}

onMounted(() => {
  document.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown)
  siteNavDrawerOpen.value = false
  if (typeof document !== 'undefined') document.body.style.overflow = ''
})
</script>

<template>
  <div class="lg:hidden">
    <button
      type="button"
      class="th-console-top-icon-btn shrink-0"
      :aria-label="t('layout.openNav')"
      :aria-expanded="open"
      @click="openDrawer"
    >
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-5 w-5" aria-hidden="true">
        <path
          fill-rule="evenodd"
          d="M2 4.75A.75.75 0 012.75 4h14.5a.75.75 0 010 1.5H2.75A.75.75 0 012 4.75zm0 5.5A.75.75 0 012.75 10h14.5a.75.75 0 010 1.5H2.75A.75.75 0 012 10.25zM2.75 15.5a.75.75 0 000 1.5h14.5a.75.75 0 000-1.5H2.75z"
          clip-rule="evenodd"
        />
      </svg>
    </button>

    <Teleport to="body">
      <Transition name="th-drawer">
        <div v-if="open" class="fixed inset-0 z-[90] flex justify-end" role="dialog" aria-modal="true" :aria-label="t('layout.mobileNavAria')">
          <button type="button" class="absolute inset-0 bg-black/55 backdrop-blur-[2px]" :aria-label="t('layout.closeNav')" @click="close" />
          <aside
            class="th-drawer-panel thin-scrollbar relative flex h-full w-[min(18rem,92vw)] flex-col overflow-y-auto border-l border-white/[0.08] bg-slate-950/98 px-4 py-5 shadow-2xl"
          >
            <div class="mb-4 flex items-center justify-between gap-2">
              <p class="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">{{ t('layout.mobileNavAria') }}</p>
              <button type="button" class="th-console-top-icon-btn shrink-0" :aria-label="t('layout.closeNav')" @click="close">
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-5 w-5" aria-hidden="true">
                  <path
                    d="M6.28 5.22a.75.75 0 00-1.06 1.06L8.94 10l-3.72 3.72a.75.75 0 101.06 1.06L10 11.06l3.72 3.72a.75.75 0 101.06-1.06L11.06 10l3.72-3.72a.75.75 0 00-1.06-1.06L10 8.94 6.28 5.22z"
                  />
                </svg>
              </button>
            </div>
            <nav class="flex flex-col gap-0.5 text-sm" :aria-label="t('layout.mobileNavAria')">
              <a href="#why" class="th-site-nav-link rounded-lg px-3 py-2" @click="onNavClick">{{ t('nav.whyTraceHub') }}</a>
              <a href="#steps" class="th-site-nav-link rounded-lg px-3 py-2" @click="onNavClick">{{ t('nav.quickStart') }}</a>
              <a href="#routing" class="th-site-nav-link rounded-lg px-3 py-2" @click="onNavClick">{{ t('nav.routing') }}</a>
              <a href="#features" class="th-site-nav-link rounded-lg px-3 py-2" @click="onNavClick">{{ t('nav.features') }}</a>
              <a href="#integration" class="th-site-nav-link rounded-lg px-3 py-2" @click="onNavClick">{{ t('nav.integration') }}</a>
              <a href="#models" class="th-site-nav-link rounded-lg px-3 py-2" @click="onNavClick">{{ t('nav.models') }}</a>
              <a href="#pricing" class="th-site-nav-link rounded-lg px-3 py-2" @click="onNavClick">{{ t('nav.pricing') }}</a>
              <a href="#faq" class="th-site-nav-link rounded-lg px-3 py-2" @click="onNavClick">{{ t('nav.faq') }}</a>
              <RouterLink to="/docs" class="th-site-nav-link rounded-lg px-3 py-2" @click="onNavClick">{{ t('nav.docs') }}</RouterLink>
            </nav>
            <div class="mt-4 flex flex-col gap-2 border-t border-white/[0.08] pt-4">
              <RegisterPromoDropdown v-if="!auth.isLoggedIn" :text="siteMeta.registerPromoText" />
              <template v-if="!auth.isLoggedIn">
                <RouterLink to="/login" class="th-btn-secondary mx-3 text-center text-sm no-underline" @click="onNavClick">{{ t('nav.login') }}</RouterLink>
                <RouterLink to="/register" class="th-btn-primary mx-3 text-center text-sm no-underline" @click="onNavClick">{{ t('nav.register') }}</RouterLink>
              </template>
              <UserAccountMenu v-else accent="cyan" />
            </div>
          </aside>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>
