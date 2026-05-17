<script setup lang="ts">
import { inject, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useAdminModelIdCombobox } from '../composables/useAdminModelIdCombobox'
import { adminSiteSettingsKey } from '../composables/useAdminSiteSettings'

const { t } = useI18n()
const route = useRoute()
const s = inject(adminSiteSettingsKey)!

type DraftRow = { model_name: string }

const draftRows = ref<DraftRow[]>([])

const flashMsg = ref('')
const flashIsError = ref(false)

function clearFlash() {
  flashMsg.value = ''
  flashIsError.value = false
}

function flashWarn(text: string) {
  flashIsError.value = true
  flashMsg.value = text
  void nextTick(() => {
    document.getElementById('no-bill-flash')?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
  })
}

function normLines(text: string): string {
  return text
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter(Boolean)
    .join('\n')
}

function rowsFromText(text: string): DraftRow[] {
  return text
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter(Boolean)
    .map((model_name) => ({ model_name }))
}

function textFromRows(rows: DraftRow[]): string {
  return rows
    .map((r) => r.model_name.trim())
    .filter(Boolean)
    .join('\n')
}

function syncDraftFromForm() {
  draftRows.value = rowsFromText(s.form.noBillModelsText)
}

/** 将表格写回表单（供底部「保存」提交）；去掉空行 */
function flushDraftToForm() {
  const next = textFromRows(draftRows.value)
  if (normLines(next) === normLines(s.form.noBillModelsText)) return
  s.form.noBillModelsText = next
}

let syncingFromForm = false
watch(
  () => s.form.noBillModelsText,
  () => {
    if (route.name !== 'admin-billing-no-bill') return
    const fromForm = normLines(s.form.noBillModelsText)
    const fromDraft = normLines(textFromRows(draftRows.value))
    if (fromForm === fromDraft) return
    syncingFromForm = true
    draftRows.value = rowsFromText(s.form.noBillModelsText)
    void nextTick(() => {
      syncingFromForm = false
    })
  },
)

function checkDuplicateIds() {
  const seen = new Set<string>()
  for (const r of draftRows.value) {
    const m = r.model_name.trim()
    if (!m) continue
    if (seen.has(m)) {
      flashWarn(t('adminNoBill.warnDuplicate', { id: m }))
      return
    }
    seen.add(m)
  }
  clearFlash()
}

watch(
  draftRows,
  () => {
    if (syncingFromForm) return
    flushDraftToForm()
    checkDuplicateIds()
  },
  { deep: true },
)

const {
  modelSuggestLoading,
  openModelSuggestRow,
  modelSuggestPanelStyle,
  setComboboxRef,
  filteredOpenModelSuggestIds,
  scheduleSuggestLayout,
  toggleModelSuggestRow,
  focusModelSuggestRow,
  pickOpenModelSuggestId,
  loadModelSuggestions,
} = useAdminModelIdCombobox(draftRows)

function addRow() {
  clearFlash()
  draftRows.value.push({ model_name: '' })
}

function removeRow(idx: number) {
  clearFlash()
  draftRows.value.splice(idx, 1)
}

onMounted(() => {
  syncDraftFromForm()
  void loadModelSuggestions()
})

watch(
  () => route.name,
  (n) => {
    if (n === 'admin-billing-no-bill') {
      syncDraftFromForm()
      void loadModelSuggestions()
    }
  },
)
</script>

<template>
  <div>
    <article
      class="overflow-hidden rounded-2xl border border-slate-800/90 bg-slate-900/60 ring-1 ring-white/[0.04]"
    >
      <div class="border-b border-slate-800/80 px-5 py-4">
        <h3 class="text-lg font-semibold text-white">{{ t('adminBilling.tabNoBill') }}</h3>
        <p class="mt-2 text-sm text-slate-400">{{ t('adminSystem.secBillingHint') }}</p>
        <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.hintNoBillModels') }}</p>
      </div>

      <div class="border-t border-slate-800/80 bg-slate-950/40 px-5 py-4">
        <p class="mb-1 text-xs text-slate-500">{{ t('adminNoBill.tableHint') }}</p>
        <p class="mb-3 text-[11px] leading-relaxed text-slate-500">
          <span v-if="modelSuggestLoading">{{ t('adminGroups.modelSuggestLoading') }}</span>
          <span v-else>{{ t('adminNoBill.modelSuggestHint') }}</span>
        </p>

        <div class="hidden overflow-x-auto rounded-xl border border-slate-800 lg:block">
          <table class="w-full min-w-[28rem] border-collapse text-left text-xs">
            <thead class="bg-slate-900 text-slate-500">
              <tr>
                <th class="align-middle px-3 py-2">{{ t('adminGroups.thModelId') }}</th>
                <th class="align-middle px-3 py-2 text-right">{{ t('adminCommon.delete') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-800">
              <tr v-for="(r, idx) in draftRows" :key="idx">
                <td class="align-middle px-2 py-2 max-w-[min(100%,22rem)] overflow-hidden">
                  <div
                    class="th-model-combobox relative max-w-full min-w-0"
                    :ref="(el) => setComboboxRef(idx, el, 'desktop')"
                  >
                    <div class="flex max-w-full min-w-0 gap-1">
                      <input
                        v-model="r.model_name"
                        class="th-input h-9 min-w-0 w-0 flex-1 py-1 font-mono text-xs leading-normal"
                        autocomplete="off"
                        spellcheck="false"
                        :placeholder="t('adminGroups.modelIdPh')"
                        @focus="focusModelSuggestRow(idx)"
                        @input="scheduleSuggestLayout"
                      />
                      <button
                        type="button"
                        class="th-input flex h-9 w-9 min-w-9 shrink-0 items-center justify-center !w-9 !max-w-9 !px-0"
                        :title="t('adminGroups.modelSuggestToggle')"
                        :aria-expanded="openModelSuggestRow === idx"
                        @mousedown.prevent
                        @click="toggleModelSuggestRow(idx)"
                      >
                        <svg class="h-4 w-4 opacity-70" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                          <path
                            fill-rule="evenodd"
                            d="M5.23 7.21a.75.75 0 011.06.02L10 11.17l3.71-3.94a.75.75 0 111.08 1.04l-4.24 4.5a.75.75 0 01-1.08 0L5.21 8.27a.75.75 0 01.02-1.06z"
                            clip-rule="evenodd"
                          />
                        </svg>
                      </button>
                    </div>
                  </div>
                </td>
                <td class="align-middle px-2 py-2 text-right">
                  <button
                    type="button"
                    class="th-link-danger-plain rounded-lg px-2 py-1 text-xs hover:bg-red-500/10"
                    @click="removeRow(idx)"
                  >
                    {{ t('adminCommon.delete') }}
                  </button>
                </td>
              </tr>
              <tr v-if="!draftRows.length">
                <td colspan="2" class="px-3 py-8 text-center text-slate-500">{{ t('adminNoBill.empty') }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="space-y-3 lg:hidden">
          <p v-if="!draftRows.length" class="rounded-xl border border-dashed border-slate-700 py-8 text-center text-sm text-slate-500">
            {{ t('adminNoBill.empty') }}
          </p>
          <article
            v-for="(r, idx) in draftRows"
            :key="`nb-m-${idx}`"
            class="rounded-xl border border-slate-800 bg-slate-950/50 p-3"
          >
            <p class="mb-2 text-[10px] font-semibold uppercase tracking-wide text-slate-500">{{ t('adminGroups.thModelId') }}</p>
            <div class="th-model-combobox relative max-w-full min-w-0" :ref="(el) => setComboboxRef(idx, el, 'mobile')">
              <div class="flex max-w-full min-w-0 gap-1">
                <input
                  v-model="r.model_name"
                  class="th-input h-9 min-w-0 w-0 flex-1 py-1 font-mono text-xs leading-normal"
                  autocomplete="off"
                  spellcheck="false"
                  :placeholder="t('adminGroups.modelIdPh')"
                  @focus="focusModelSuggestRow(idx)"
                  @input="scheduleSuggestLayout"
                />
                <button
                  type="button"
                  class="th-input flex h-9 w-9 min-w-9 shrink-0 items-center justify-center !w-9 !max-w-9 !px-0"
                  :title="t('adminGroups.modelSuggestToggle')"
                  :aria-expanded="openModelSuggestRow === idx"
                  @mousedown.prevent
                  @click="toggleModelSuggestRow(idx)"
                >
                  <svg class="h-4 w-4 opacity-70" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                    <path
                      fill-rule="evenodd"
                      d="M5.23 7.21a.75.75 0 011.06.02L10 11.17l3.71-3.94a.75.75 0 111.08 1.04l-4.24 4.5a.75.75 0 01-1.08 0L5.21 8.27a.75.75 0 01.02-1.06z"
                      clip-rule="evenodd"
                    />
                  </svg>
                </button>
              </div>
            </div>
            <button
              type="button"
              class="th-link-danger-plain mt-3 rounded-lg px-2 py-1 text-xs hover:bg-red-500/10"
              @click="removeRow(idx)"
            >
              {{ t('adminCommon.delete') }}
            </button>
          </article>
        </div>

        <p
          v-if="flashMsg"
          id="no-bill-flash"
          class="mt-3 rounded-lg border px-3 py-2 text-xs"
          :class="flashIsError ? 'th-admin-flash--error' : 'th-admin-flash--success'"
          role="status"
          aria-live="polite"
        >
          {{ flashMsg }}
        </p>

        <div class="mt-3 flex flex-wrap gap-2">
          <button
            type="button"
            class="rounded-lg border border-slate-600 px-3 py-1.5 text-xs text-slate-300 hover:bg-slate-800"
            @click="addRow"
          >
            {{ t('adminGroups.addRow') }}
          </button>
        </div>

        <dl v-if="s.noBillModelsYaml.trim()" class="mt-4 th-admin-compare-tile">
          <dt class="text-[10px] uppercase tracking-wide text-slate-500">{{ t('adminSystem.lblNoBillModelsYaml') }}</dt>
          <dd class="m-0 mt-1 whitespace-pre-wrap font-mono text-[11px] leading-relaxed">{{ s.noBillModelsYaml }}</dd>
        </dl>
        <p v-if="s.noBillModelsDbOverride" class="mt-3 text-[11px] leading-relaxed text-amber-100/80">
          {{ t('adminSystem.hintNoBillModelsDb') }}
        </p>
      </div>
    </article>

    <Teleport to="body">
      <ul
        v-if="openModelSuggestRow !== null && !modelSuggestLoading"
        class="th-model-combobox-list th-model-combobox-floating thin-scrollbar"
        :style="modelSuggestPanelStyle"
        role="listbox"
        tabindex="-1"
        @wheel.stop
      >
        <template v-if="filteredOpenModelSuggestIds.length">
          <li
            v-for="opt in filteredOpenModelSuggestIds"
            :key="opt"
            class="th-model-combobox-item"
            role="option"
            @mousedown.prevent="pickOpenModelSuggestId(opt)"
          >
            {{ opt }}
          </li>
        </template>
        <li v-else class="th-model-combobox-item th-model-combobox-item--muted cursor-default">
          {{ t('adminGroups.modelSuggestEmpty') }}
        </li>
      </ul>
    </Teleport>
  </div>
</template>
