<script setup lang="ts">
import { inject, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import AdminModelChannelPickModal from '../components/admin/AdminModelChannelPickModal.vue'
import { useAdminModelIdCombobox } from '../composables/useAdminModelIdCombobox'
import { adminSiteSettingsKey } from '../composables/useAdminSiteSettings'
import { channelRefLabel, type AdminModelChannelRef } from '../utils/adminModelChannelMap'

const { t } = useI18n()
const route = useRoute()
const s = inject(adminSiteSettingsKey)!

type DraftRow = { model_name: string; channel_id?: number }

const draftRows = ref<DraftRow[]>([])
const noBillWorkspaceScope = ref<'global' | number>('global')

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
    .map((model_name) => ({ model_name, channel_id: undefined }))
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
  modelSuggestLoadingUi,
  modelSuggestTotal,
  openModelSuggestRow,
  modelSuggestPanelStyle,
  setComboboxRef,
  filteredOpenModelSuggestIds,
  suggestListTruncated,
  channelCountForModel,
  channelsForModel,
  resolveRowChannel,
  scheduleSuggestLayout,
  toggleModelSuggestRow,
  focusModelSuggestRow,
  pickOpenModelSuggestId,
  loadModelSuggestions,
  channelPickOpen,
  channelPickModelId,
  channelPickChannels,
  channelPickSelectedId,
  confirmChannelPick,
  cancelChannelPick,
} = useAdminModelIdCombobox(draftRows, {
  workspaceScope: noBillWorkspaceScope,
  enableChannelPick: true,
})

function rowChannels(r: DraftRow): AdminModelChannelRef[] {
  return channelsForModel(r.model_name.trim())
}

function rowChannelLabel(r: DraftRow): string {
  const m = r.model_name.trim()
  if (!m) return ''
  const chs = rowChannels(r)
  if (chs.length === 0) return t('adminNoBill.channelNone')
  if (r.channel_id != null) {
    const picked = chs.find((c) => c.id === r.channel_id)
    if (picked) return channelRefLabel(picked)
  }
  if (chs.length === 1) return channelRefLabel(chs[0])
  return t('adminNoBill.channelMultiPending', { n: chs.length })
}

function onModelIdInput(idx: number) {
  const r = draftRows.value[idx]
  if (!r) return
  resolveRowChannel(r)
  scheduleSuggestLayout()
}

function onRowChannelSelect(idx: number, raw: string) {
  const r = draftRows.value[idx]
  if (!r) return
  const n = Number(raw)
  r.channel_id = Number.isFinite(n) && n > 0 ? Math.trunc(n) : undefined
}

function addRow() {
  clearFlash()
  draftRows.value.push({ model_name: '', channel_id: undefined })
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
          <span v-if="modelSuggestLoadingUi">{{ t('adminGroups.modelSuggestLoading') }}</span>
          <span v-else>{{ t('adminNoBill.modelSuggestHint', { total: modelSuggestTotal }) }}</span>
        </p>

        <div class="hidden overflow-x-auto rounded-xl border border-slate-800 lg:block">
          <table class="w-full min-w-[28rem] border-collapse text-left text-xs">
            <thead class="bg-slate-900 text-slate-500">
              <tr>
                <th class="align-middle px-3 py-2">{{ t('adminGroups.thModelId') }}</th>
                <th class="align-middle px-3 py-2">{{ t('adminNoBill.thChannel') }}</th>
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
                        @input="onModelIdInput(idx)"
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
                <td class="align-middle px-2 py-2 min-w-[10rem] max-w-[18rem]">
                  <template v-if="!r.model_name.trim()">
                    <span class="text-slate-600">{{ t('adminCommon.dash') }}</span>
                  </template>
                  <select
                    v-else-if="rowChannels(r).length > 1"
                    class="th-select th-select--table-cell w-full max-w-full"
                    :value="r.channel_id ?? ''"
                    @change="onRowChannelSelect(idx, ($event.target as HTMLSelectElement).value)"
                  >
                    <option value="">{{ t('adminNoBill.channelSelectPh') }}</option>
                    <option v-for="ch in rowChannels(r)" :key="ch.id" :value="ch.id">
                      {{ channelRefLabel(ch) }}{{ ch.healthy ? '' : ` (${t('adminModelChannelPick.unhealthy')})` }}
                    </option>
                  </select>
                  <span
                    v-else
                    class="th-no-bill-channel-label block truncate text-xs leading-9 text-slate-400"
                    :class="rowChannels(r).length === 0 ? 'text-slate-600' : ''"
                    :title="rowChannelLabel(r)"
                  >{{ rowChannelLabel(r) }}</span>
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
                <td colspan="3" class="px-3 py-8 text-center text-slate-500">{{ t('adminNoBill.empty') }}</td>
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
                  @input="onModelIdInput(idx)"
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
            <p class="mb-1 mt-3 text-[10px] font-semibold uppercase tracking-wide text-slate-500">{{ t('adminNoBill.thChannel') }}</p>
            <template v-if="!r.model_name.trim()">
              <p class="text-sm text-slate-600">{{ t('adminCommon.dash') }}</p>
            </template>
            <select
              v-else-if="rowChannels(r).length > 1"
              class="th-select th-select--table-cell w-full"
              :value="r.channel_id ?? ''"
              @change="onRowChannelSelect(idx, ($event.target as HTMLSelectElement).value)"
            >
              <option value="">{{ t('adminNoBill.channelSelectPh') }}</option>
              <option v-for="ch in rowChannels(r)" :key="ch.id" :value="ch.id">
                {{ channelRefLabel(ch) }}{{ ch.healthy ? '' : ` (${t('adminModelChannelPick.unhealthy')})` }}
              </option>
            </select>
            <p v-else class="th-no-bill-channel-label truncate text-xs leading-9 text-slate-400" :title="rowChannelLabel(r)">
              {{ rowChannelLabel(r) }}
            </p>
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
            class="th-model-combobox-item flex items-center justify-between gap-2"
            role="option"
            @mousedown.prevent="pickOpenModelSuggestId(opt)"
          >
            <span class="min-w-0 truncate font-mono">{{ opt }}</span>
            <span
              v-if="channelCountForModel(opt) > 1"
              class="shrink-0 text-[10px] text-amber-300/90"
            >{{ t('adminModelChannelPick.multiBadge', { n: channelCountForModel(opt) }) }}</span>
          </li>
          <li
            v-if="suggestListTruncated"
            class="th-model-combobox-item th-model-combobox-item--muted cursor-default"
          >
            {{ t('adminModelChannelPick.suggestTruncate', { total: modelSuggestTotal }) }}
          </li>
        </template>
        <li v-else class="th-model-combobox-item th-model-combobox-item--muted cursor-default">
          {{ t('adminGroups.modelSuggestEmpty') }}
        </li>
      </ul>
    </Teleport>

    <AdminModelChannelPickModal
      :open="channelPickOpen"
      :model-id="channelPickModelId"
      :channels="channelPickChannels"
      :selected-channel-id="channelPickSelectedId"
      @update:selected-channel-id="channelPickSelectedId = $event"
      @confirm="confirmChannelPick"
      @cancel="cancelChannelPick"
    />
  </div>
</template>
