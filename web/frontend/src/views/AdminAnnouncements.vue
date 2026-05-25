<script setup lang="ts">
import axios from 'axios'
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import MobileLabelValueCard from '../components/ui/MobileLabelValueCard.vue'
import MobileLabelValueRow from '../components/ui/MobileLabelValueRow.vue'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import ThTableSkeleton from '../components/ui/ThTableSkeleton.vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'

const { t } = useI18n()

type Row = {
  id: number
  title: string
  content: string
  published: boolean
  published_at?: string
  created_at?: string
  updated_at?: string
  audience_user_id?: number | null
}

const rows = ref<Row[]>([])
const total = ref(0)
const { busy: loading, visible: loadingUi, run: runLoad } = useSmoothLoading()
const page = ref(1)
const pageSize = 20
const msg = ref('')
const err = ref('')

const createForm = reactive({ title: '', content: '' })
const editOpen = ref(false)
const editForm = reactive({ id: 0, title: '', content: '' })

function httpErr(e: unknown): string {
  if (axios.isAxiosError(e) && e.response?.data && typeof e.response.data === 'object') {
    const d = e.response.data as { message?: string }
    if (typeof d.message === 'string' && d.message.trim()) return d.message.trim()
  }
  return e instanceof Error ? e.message : t('adminCommon.requestFail')
}

async function load() {
  err.value = ''
  await runLoad(async () => {
    const { data } = await http.get<Envelope<{ items: Row[]; total: number }>>('/api/admin/announcements', {
      params: { page: page.value, page_size: pageSize },
    })
    if (data.code !== 0) throw new Error(data.message || t('adminCommon.loadFail'))
    rows.value = data.data.items || []
    total.value = data.data.total ?? 0
  }).catch((e: unknown) => {
    err.value = httpErr(e)
  })
}

async function createDraft() {
  msg.value = ''
  err.value = ''
  if (!createForm.title.trim() || !createForm.content.trim()) {
    err.value = t('adminAnnounce.fillRequired')
    return
  }
  try {
    await http.post('/api/admin/announcements', {
      title: createForm.title.trim(),
      content: createForm.content.trim(),
    })
    createForm.title = ''
    createForm.content = ''
    msg.value = t('adminAnnounce.created')
    await load()
  } catch (e: unknown) {
    err.value = httpErr(e)
  }
}

async function createAndPublish() {
  msg.value = ''
  err.value = ''
  if (!createForm.title.trim() || !createForm.content.trim()) {
    err.value = t('adminAnnounce.fillRequired')
    return
  }
  try {
    await http.post('/api/admin/announcements', {
      title: createForm.title.trim(),
      content: createForm.content.trim(),
      publish: true,
    })
    createForm.title = ''
    createForm.content = ''
    msg.value = t('adminAnnounce.createdPublished')
    await load()
  } catch (e: unknown) {
    err.value = httpErr(e)
  }
}

function openEdit(r: Row) {
  editForm.id = r.id
  editForm.title = r.title
  editForm.content = r.content
  editOpen.value = true
}

async function saveEdit() {
  err.value = ''
  try {
    await http.put(`/api/admin/announcements/${editForm.id}`, {
      title: editForm.title.trim(),
      content: editForm.content.trim(),
    })
    editOpen.value = false
    msg.value = t('adminAnnounce.saved')
    await load()
  } catch (e: unknown) {
    err.value = httpErr(e)
  }
}

async function publish(id: number) {
  err.value = ''
  try {
    await http.post(`/api/admin/announcements/${id}/publish`)
    msg.value = t('adminAnnounce.published')
    await load()
  } catch (e: unknown) {
    err.value = httpErr(e)
  }
}

async function unpublish(id: number) {
  err.value = ''
  try {
    await http.post(`/api/admin/announcements/${id}/unpublish`)
    msg.value = t('adminAnnounce.unpublished')
    await load()
  } catch (e: unknown) {
    err.value = httpErr(e)
  }
}

async function remove(id: number) {
  if (!confirm(t('common.confirmDelete'))) return
  err.value = ''
  try {
    await http.delete(`/api/admin/announcements/${id}`)
    msg.value = t('adminAnnounce.deleted')
    await load()
  } catch (e: unknown) {
    err.value = httpErr(e)
  }
}

function prevPage() {
  if (page.value > 1) {
    page.value -= 1
    void load()
  }
}

function nextPage() {
  if (page.value * pageSize < total.value) {
    page.value += 1
    void load()
  }
}

onMounted(() => void load())
</script>

<template>
  <div class="th-admin-announce-root flex min-h-0 flex-1 flex-col gap-6">
    <div>
      <h1 class="text-lg font-semibold text-slate-100">{{ t('adminAnnounce.title') }}</h1>
      <p class="mt-1 text-sm text-slate-500">{{ t('adminAnnounce.desc') }}</p>
    </div>

    <p v-if="msg" class="th-admin-flash--success rounded-lg border px-3 py-2 text-sm">{{ msg }}</p>
    <p v-if="err" class="th-admin-flash--error rounded-lg border px-3 py-2 text-sm">{{ err }}</p>

    <section class="th-glass rounded-[var(--th-radius-lg)] p-5">
      <h2 class="text-sm font-medium text-slate-200">{{ t('adminAnnounce.newDraft') }}</h2>
      <div class="mt-3 flex flex-col gap-3">
        <input
          v-model="createForm.title"
          type="text"
          maxlength="500"
          class="th-input w-full text-sm"
          :placeholder="t('adminAnnounce.titlePh')"
        />
        <textarea
          v-model="createForm.content"
          rows="4"
          class="th-input w-full resize-y text-sm"
          :placeholder="t('adminAnnounce.contentPh')"
        />
        <div class="flex flex-wrap gap-2">
          <button type="button" class="th-btn-secondary px-4 py-2 text-sm" @click="createDraft">
            {{ t('adminAnnounce.saveDraft') }}
          </button>
          <button type="button" class="th-btn-primary px-4 py-2 text-sm" @click="createAndPublish">
            {{ t('adminAnnounce.publishNow') }}
          </button>
        </div>
      </div>
    </section>

    <section class="th-glass min-h-0 flex-1 rounded-[var(--th-radius-lg)] p-5">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 class="text-sm font-medium text-slate-200">{{ t('adminAnnounce.list') }}</h2>
        <span class="text-xs text-slate-500">{{ t('adminAnnounce.total', { n: total }) }}</span>
      </div>
      <ThTableSkeleton v-if="loadingUi && rows.length === 0" :rows="8" :columns="6" class="mb-4" />
      <div v-if="loadingUi && rows.length === 0" class="py-8 lg:hidden">
        <ThLoadingState />
      </div>
      <div class="th-admin-table-shell relative hidden lg:block">
        <ThLoadingState v-if="loadingUi && rows.length > 0" variant="overlay" />
        <table class="w-full min-w-[640px] text-left text-sm">
          <thead class="th-admin-table-head">
            <tr class="text-xs uppercase tracking-wide text-slate-500">
              <th class="px-3 py-2">{{ t('adminAnnounce.thId') }}</th>
              <th class="px-3 py-2">{{ t('adminAnnounce.thTitle') }}</th>
              <th class="px-3 py-2">{{ t('adminAnnounce.thAudience') }}</th>
              <th class="px-3 py-2">{{ t('adminAnnounce.thStatus') }}</th>
              <th class="px-3 py-2">{{ t('adminAnnounce.thPublishedAt') }}</th>
              <th class="px-3 py-2 text-right">{{ t('adminAnnounce.thActions') }}</th>
            </tr>
          </thead>
          <tbody class="th-admin-table-body">
            <tr v-for="r in rows" :key="r.id" class="text-slate-300">
              <td class="px-3 py-2.5 font-mono text-xs text-slate-400">{{ r.id }}</td>
              <td class="max-w-[220px] truncate px-3 py-2.5" :title="r.title">{{ r.title }}</td>
              <td class="px-3 py-2.5 text-xs text-slate-400">
                <span v-if="r.audience_user_id != null && r.audience_user_id > 0">{{ t('adminAnnounce.audienceUser', { id: r.audience_user_id }) }}</span>
                <span v-else>{{ t('adminAnnounce.audienceAll') }}</span>
              </td>
              <td class="px-3 py-2.5">
                <span v-if="r.published" class="th-badge-apikey th-badge-apikey--enabled">{{ t('adminAnnounce.statusOn') }}</span>
                <span v-else class="text-slate-500">{{ t('adminAnnounce.statusDraft') }}</span>
              </td>
              <td class="px-3 py-2.5 text-xs text-slate-400">{{ r.published_at || '—' }}</td>
              <td class="px-3 py-2.5 text-right">
                <button type="button" class="th-link-accent mr-2 text-xs hover:underline" @click="openEdit(r)">{{ t('common.edit') }}</button>
                <button v-if="!r.published" type="button" class="th-link-accent mr-2 text-xs hover:underline" @click="publish(r.id)">
                  {{ t('adminAnnounce.publish') }}
                </button>
                <button v-else type="button" class="mr-2 text-xs text-slate-500 hover:underline" @click="unpublish(r.id)">
                  {{ t('adminAnnounce.unpublish') }}
                </button>
                <button type="button" class="th-link-danger-plain text-xs" @click="remove(r.id)">{{ t('common.delete') }}</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="mt-3 space-y-3 lg:hidden">
        <MobileLabelValueCard v-for="r in rows" :key="`ann-m-${r.id}`" :title="r.title">
          <MobileLabelValueRow :label="t('adminAnnounce.thId')">{{ r.id }}</MobileLabelValueRow>
          <MobileLabelValueRow :label="t('adminAnnounce.thAudience')">
            <span v-if="r.audience_user_id != null && r.audience_user_id > 0">{{ t('adminAnnounce.audienceUser', { id: r.audience_user_id }) }}</span>
            <span v-else>{{ t('adminAnnounce.audienceAll') }}</span>
          </MobileLabelValueRow>
          <MobileLabelValueRow :label="t('adminAnnounce.thStatus')">
            <span v-if="r.published" class="th-badge-apikey th-badge-apikey--enabled">{{ t('adminAnnounce.statusOn') }}</span>
            <span v-else class="text-slate-500">{{ t('adminAnnounce.statusDraft') }}</span>
          </MobileLabelValueRow>
          <MobileLabelValueRow :label="t('adminAnnounce.thPublishedAt')">{{ r.published_at || '—' }}</MobileLabelValueRow>
          <div class="flex flex-wrap gap-2 pt-1">
            <button type="button" class="th-link-accent text-xs hover:underline" @click="openEdit(r)">{{ t('common.edit') }}</button>
            <button v-if="!r.published" type="button" class="th-link-accent text-xs hover:underline" @click="publish(r.id)">{{ t('adminAnnounce.publish') }}</button>
            <button v-else type="button" class="text-xs text-slate-500 hover:underline" @click="unpublish(r.id)">{{ t('adminAnnounce.unpublish') }}</button>
            <button type="button" class="th-link-danger-plain text-xs" @click="remove(r.id)">{{ t('common.delete') }}</button>
          </div>
        </MobileLabelValueCard>
      </div>
      <div class="mt-4 flex items-center justify-end gap-2">
        <button type="button" class="th-admin-toolbar-btn" :disabled="page <= 1" @click="prevPage">
          {{ t('adminCommon.prevPage') }}
        </button>
        <button type="button" class="th-admin-toolbar-btn" :disabled="page * pageSize >= total" @click="nextPage">
          {{ t('adminCommon.nextPage') }}
        </button>
      </div>
    </section>

    <Teleport to="body">
      <div
        v-if="editOpen"
        class="th-feedback-backdrop fixed inset-0 z-[100] flex items-center justify-center p-4"
        role="dialog"
        aria-modal="true"
        @click.self="editOpen = false"
      >
        <div class="th-feedback-modal w-full max-w-lg rounded-xl p-5">
          <h3 class="th-feedback-modal__title text-sm font-semibold">{{ t('adminAnnounce.editTitle', { id: editForm.id }) }}</h3>
          <div class="mt-4 flex flex-col gap-3">
            <input v-model="editForm.title" type="text" maxlength="500" class="th-input w-full text-sm" />
            <textarea v-model="editForm.content" rows="6" class="th-input w-full resize-y text-sm" />
          </div>
          <div class="mt-5 flex justify-end gap-2">
            <button type="button" class="th-modal-secondary-btn px-3 py-1.5 text-sm" @click="editOpen = false">
              {{ t('adminCommon.cancel') }}
            </button>
            <button type="button" class="th-btn-primary-flat px-4 py-1.5 text-sm" @click="saveEdit">
              {{ t('adminCommon.save') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
