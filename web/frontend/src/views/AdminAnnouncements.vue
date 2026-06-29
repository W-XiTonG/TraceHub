<script setup lang="ts">
import axios from 'axios'
import { onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import AdminAnnounceUserPicker from '../components/admin/AdminAnnounceUserPicker.vue'
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
  audience?: 'all' | 'specific'
  audience_user_id?: number | null
  audience_user_ids?: number[]
}

type EmailNotify = {
  requested?: boolean
  skipped_reason?: string
  recipient_count?: number
}

const rows = ref<Row[]>([])
const total = ref(0)
const { busy: loading, visible: loadingUi, run: runLoad } = useSmoothLoading()
const page = ref(1)
const pageSize = 20
const msg = ref('')
const err = ref('')

const createForm = reactive({
  title: '',
  content: '',
  audience: 'all' as 'all' | 'specific',
  notifyEmail: false,
})
const createSelectedUserIds = ref<number[]>([])

const editOpen = ref(false)
const editForm = reactive({
  id: 0,
  title: '',
  content: '',
  published: false,
  audience: 'all' as 'all' | 'specific',
})
const editSelectedUserIds = ref<number[]>([])

const publishOpen = ref(false)
const publishTargetId = ref(0)
const publishNotifyEmail = ref(true)

watch(
  () => createForm.audience,
  (v) => {
    if (v === 'all') createSelectedUserIds.value = []
  },
)

watch(
  () => editForm.audience,
  (v) => {
    if (v === 'all') editSelectedUserIds.value = []
  },
)

function httpErr(e: unknown): string {
  if (axios.isAxiosError(e) && e.response?.data && typeof e.response.data === 'object') {
    const d = e.response.data as { message?: string }
    if (typeof d.message === 'string' && d.message.trim()) return d.message.trim()
  }
  return e instanceof Error ? e.message : t('adminCommon.requestFail')
}

function audienceLabel(r: Row): string {
  if (r.audience_user_id != null && r.audience_user_id > 0) {
    return t('adminAnnounce.audienceUser', { id: r.audience_user_id })
  }
  const n = r.audience_user_ids?.length ?? 0
  if (n > 0) return t('adminAnnounce.audienceCount', { n })
  return t('adminAnnounce.audienceAll')
}

function applyEmailNotifyHint(en?: EmailNotify) {
  if (!en?.requested) return
  if (en.skipped_reason === 'smtp_not_configured') {
    msg.value = t('adminAnnounce.emailSkippedSmtp')
    return
  }
  if (en.skipped_reason === 'no_recipients') {
    msg.value = t('adminAnnounce.emailSkippedNoRecipients')
    return
  }
  if (en.recipient_count && en.recipient_count > 0) {
    msg.value = t('adminAnnounce.emailQueued', { n: en.recipient_count })
  }
}

function audiencePayload(audience: 'all' | 'specific', ids: number[]) {
  if (audience === 'all') return { audience: 'all' as const }
  if (!ids.length) throw new Error(t('adminAnnounce.audienceRequired'))
  return { audience: 'specific' as const, audience_user_ids: ids }
}

function resetCreateForm() {
  createForm.title = ''
  createForm.content = ''
  createForm.audience = 'all'
  createForm.notifyEmail = false
  createSelectedUserIds.value = []
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
    const aud = audiencePayload(createForm.audience, createSelectedUserIds.value)
    await http.post('/api/admin/announcements', {
      title: createForm.title.trim(),
      content: createForm.content.trim(),
      ...aud,
    })
    resetCreateForm()
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
    const aud = audiencePayload(createForm.audience, createSelectedUserIds.value)
    const notifyEmail = createForm.notifyEmail
    const { data } = await http.post<Envelope<Row & { email_notify?: EmailNotify }>>('/api/admin/announcements', {
      title: createForm.title.trim(),
      content: createForm.content.trim(),
      publish: true,
      notify_email: notifyEmail,
      ...aud,
    })
    resetCreateForm()
    if (notifyEmail) {
      applyEmailNotifyHint(data.data?.email_notify)
    }
    if (!msg.value) msg.value = t('adminAnnounce.createdPublished')
    await load()
  } catch (e: unknown) {
    err.value = httpErr(e)
  }
}

function openEdit(r: Row) {
  editForm.id = r.id
  editForm.title = r.title
  editForm.content = r.content
  editForm.published = r.published
  if (r.audience_user_id != null && r.audience_user_id > 0) {
    editForm.audience = 'specific'
    editSelectedUserIds.value = [r.audience_user_id]
  } else if (r.audience_user_ids?.length || r.audience === 'specific') {
    editForm.audience = 'specific'
    editSelectedUserIds.value = [...(r.audience_user_ids ?? [])]
  } else {
    editForm.audience = 'all'
    editSelectedUserIds.value = []
  }
  editOpen.value = true
}

async function saveEdit() {
  err.value = ''
  try {
    const body: Record<string, unknown> = {
      title: editForm.title.trim(),
      content: editForm.content.trim(),
    }
    if (!editForm.published) {
      Object.assign(body, audiencePayload(editForm.audience, editSelectedUserIds.value))
    }
    await http.put(`/api/admin/announcements/${editForm.id}`, body)
    editOpen.value = false
    msg.value = t('adminAnnounce.saved')
    await load()
  } catch (e: unknown) {
    err.value = httpErr(e)
  }
}

function openPublishModal(id: number) {
  publishTargetId.value = id
  publishNotifyEmail.value = true
  publishOpen.value = true
}

async function confirmPublish() {
  err.value = ''
  try {
    const { data } = await http.post<Envelope<Row & { email_notify?: EmailNotify }>>(
      `/api/admin/announcements/${publishTargetId.value}/publish`,
      { notify_email: publishNotifyEmail.value },
    )
    publishOpen.value = false
    if (publishNotifyEmail.value) {
      applyEmailNotifyHint(data.data?.email_notify)
    }
    if (!msg.value) msg.value = t('adminAnnounce.published')
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

        <fieldset class="flex flex-col gap-2">
          <legend class="text-xs font-medium text-slate-400">{{ t('adminAnnounce.lblAudience') }}</legend>
          <label class="flex items-center gap-2 text-sm text-slate-300">
            <input v-model="createForm.audience" type="radio" value="all" class="accent-[var(--th-accent)]" />
            {{ t('adminAnnounce.audienceAll') }}
          </label>
          <label class="flex items-center gap-2 text-sm text-slate-300">
            <input v-model="createForm.audience" type="radio" value="specific" class="accent-[var(--th-accent)]" />
            {{ t('adminAnnounce.audienceSpecific') }}
          </label>
        </fieldset>

        <div v-if="createForm.audience === 'specific'" class="rounded-lg border border-slate-700/80 bg-slate-950/40 p-3">
          <AdminAnnounceUserPicker v-model="createSelectedUserIds" />
        </div>

        <label class="flex items-start gap-2 text-sm text-slate-300">
          <input v-model="createForm.notifyEmail" type="checkbox" class="mt-0.5 accent-[var(--th-accent)]" />
          <span>
            {{ t('adminAnnounce.notifyEmail') }}
            <span class="mt-0.5 block text-xs text-slate-500">{{ t('adminAnnounce.notifyEmailHint') }}</span>
          </span>
        </label>

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
              <td class="px-3 py-2.5 text-xs text-slate-400">{{ audienceLabel(r) }}</td>
              <td class="px-3 py-2.5">
                <span v-if="r.published" class="th-badge-apikey th-badge-apikey--enabled">{{ t('adminAnnounce.statusOn') }}</span>
                <span v-else class="text-slate-500">{{ t('adminAnnounce.statusDraft') }}</span>
              </td>
              <td class="px-3 py-2.5 text-xs text-slate-400">{{ r.published_at || '—' }}</td>
              <td class="px-3 py-2.5 text-right">
                <button type="button" class="th-link-accent mr-2 text-xs hover:underline" @click="openEdit(r)">{{ t('common.edit') }}</button>
                <button v-if="!r.published" type="button" class="th-link-accent mr-2 text-xs hover:underline" @click="openPublishModal(r.id)">
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
          <MobileLabelValueRow :label="t('adminAnnounce.thAudience')">{{ audienceLabel(r) }}</MobileLabelValueRow>
          <MobileLabelValueRow :label="t('adminAnnounce.thStatus')">
            <span v-if="r.published" class="th-badge-apikey th-badge-apikey--enabled">{{ t('adminAnnounce.statusOn') }}</span>
            <span v-else class="text-slate-500">{{ t('adminAnnounce.statusDraft') }}</span>
          </MobileLabelValueRow>
          <MobileLabelValueRow :label="t('adminAnnounce.thPublishedAt')">{{ r.published_at || '—' }}</MobileLabelValueRow>
          <div class="flex flex-wrap gap-2 pt-1">
            <button type="button" class="th-link-accent text-xs hover:underline" @click="openEdit(r)">{{ t('common.edit') }}</button>
            <button v-if="!r.published" type="button" class="th-link-accent text-xs hover:underline" @click="openPublishModal(r.id)">{{ t('adminAnnounce.publish') }}</button>
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
        <div class="th-feedback-modal max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-xl p-5">
          <h3 class="th-feedback-modal__title text-sm font-semibold">{{ t('adminAnnounce.editTitle', { id: editForm.id }) }}</h3>
          <div class="mt-4 flex flex-col gap-3">
            <input v-model="editForm.title" type="text" maxlength="500" class="th-input w-full text-sm" />
            <textarea v-model="editForm.content" rows="6" class="th-input w-full resize-y text-sm" />

            <template v-if="!editForm.published">
              <fieldset class="flex flex-col gap-2">
                <legend class="text-xs font-medium text-slate-400">{{ t('adminAnnounce.lblAudience') }}</legend>
                <label class="flex items-center gap-2 text-sm text-slate-300">
                  <input v-model="editForm.audience" type="radio" value="all" class="accent-[var(--th-accent)]" />
                  {{ t('adminAnnounce.audienceAll') }}
                </label>
                <label class="flex items-center gap-2 text-sm text-slate-300">
                  <input v-model="editForm.audience" type="radio" value="specific" class="accent-[var(--th-accent)]" />
                  {{ t('adminAnnounce.audienceSpecific') }}
                </label>
              </fieldset>
              <div v-if="editForm.audience === 'specific'" class="rounded-lg border border-slate-700/80 bg-slate-950/40 p-3">
                <AdminAnnounceUserPicker :key="`edit-picker-${editForm.id}`" v-model="editSelectedUserIds" />
              </div>
            </template>
            <p v-else class="text-xs text-slate-500">{{ t('adminAnnounce.audienceLocked') }}</p>
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

      <div
        v-if="publishOpen"
        class="th-feedback-backdrop fixed inset-0 z-[100] flex items-center justify-center p-4"
        role="dialog"
        aria-modal="true"
        @click.self="publishOpen = false"
      >
        <div class="th-feedback-modal w-full max-w-md rounded-xl p-5">
          <h3 class="th-feedback-modal__title text-sm font-semibold">{{ t('adminAnnounce.publishModalTitle') }}</h3>
          <p class="mt-2 text-xs text-slate-500">{{ t('adminAnnounce.publishModalHint') }}</p>
          <label class="mt-4 flex items-start gap-2 text-sm text-slate-300">
            <input v-model="publishNotifyEmail" type="checkbox" class="mt-0.5 accent-[var(--th-accent)]" />
            <span>{{ t('adminAnnounce.notifyEmail') }}</span>
          </label>
          <div class="mt-5 flex justify-end gap-2">
            <button type="button" class="th-modal-secondary-btn px-3 py-1.5 text-sm" @click="publishOpen = false">
              {{ t('adminCommon.cancel') }}
            </button>
            <button type="button" class="th-btn-primary-flat px-4 py-1.5 text-sm" @click="confirmPublish">
              {{ t('adminAnnounce.publish') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
