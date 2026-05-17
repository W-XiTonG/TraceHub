<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const open = defineModel<boolean>({ required: true })

const { accent = 'cyan' } = defineProps<{
  accent?: 'cyan' | 'amber'
}>()

function close() {
  open.value = false
}
</script>

<template>
  <Teleport to="body">
    <Transition name="th-drawer">
      <div v-if="open" class="fixed inset-0 z-[90] flex" role="dialog" aria-modal="true" :aria-label="t('layout.mobileNavAria')">
        <button
          type="button"
          class="absolute inset-0 bg-black/55 backdrop-blur-[2px]"
          :aria-label="t('layout.closeNav')"
          @click="close"
        />
        <aside
          class="th-drawer-panel th-drawer-panel--from-left th-sidebar thin-scrollbar relative flex h-full w-[min(17.5rem,88vw)] shrink-0 flex-col overflow-y-auto px-4 py-6 shadow-2xl"
          :class="accent === 'amber' ? 'th-sidebar--drawer-amber' : ''"
        >
          <div class="mb-4 flex items-center justify-between gap-2">
            <p class="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">{{ t('layout.mobileNavAria') }}</p>
            <button
              type="button"
              class="th-console-top-icon-btn shrink-0"
              :aria-label="t('layout.closeNav')"
              @click="close"
            >
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-5 w-5" aria-hidden="true">
                <path
                  d="M6.28 5.22a.75.75 0 00-1.06 1.06L8.94 10l-3.72 3.72a.75.75 0 101.06 1.06L10 11.06l3.72 3.72a.75.75 0 101.06-1.06L11.06 10l3.72-3.72a.75.75 0 00-1.06-1.06L10 8.94 6.28 5.22z"
                />
              </svg>
            </button>
          </div>
          <slot />
        </aside>
      </div>
    </Transition>
  </Teleport>
</template>
