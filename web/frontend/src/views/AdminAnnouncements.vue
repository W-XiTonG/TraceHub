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
  <div class="flex min-h-0 flex-1 flex-col gap-6">
    <div>
      <h1 class="text-lg font-semibold text-slate-100">{{ t('adminAnnounce.title') }}</h1>
      <p class="mt-1 text-sm text-slate-500">{{ t('adminAnnounce.desc') }}</p>
    </div>

    <p v-if="msg" class="rounded-lg border border-emerald-500/25 bg-emerald-950/30 px-3 py-2 text-sm text-emerald-100/90">{{ msg }}</p>
    <p v-if="err" class="rounded-lg border border-rose-500/25 bg-rose-950/30 px-3 py-2 text-sm text-rose-100/90">{{ err }}</p>

    <section class="rounded-xl border border-white/[0.08] bg-white/[0.03] p-5">
      <h2 class="text-sm font-medium text-slate-200">{{ t('adminAnnounce.newDraft') }}</h2>
      <div class="mt-3 flex flex-col gap-3">
        <input
          v-model="createForm.title"
          type="text"
          maxlength="500"
          class="w-full rounded-lg border border-white/[0.1] bg-slate-950/50 px-3 py-2 text-sm text-slate-100 outline-none focus:border-amber-500/40"
          :placeholder="t('adminAnnounce.titlePh')"
        />
        <textarea
          v-model="createForm.content"
          rows="4"
          class="w-full resize-y rounded-lg border border-white/[0.1] bg-slate-950/50 px-3 py-2 text-sm text-slate-100 outline-none focus:border-amber-500/40"
          :placeholder="t('adminAnnounce.contentPh')"
        />
        <div class="flex flex-wrap gap-2">
          <button
            type="button"
            class="rounded-lg border border-white/[0.12] bg-white/[0.06] px-4 py-2 text-sm font-medium text-slate-200 transition hover:bg-white/[0.1]"
            @click="createDraft"
          >
            {{ t('adminAnnounce.saveDraft') }}
          </button>
          <button
            type="button"
            class="rounded-lg bg-emerald-600/90 px-4 py-2 text-sm font-medium text-emerald-50 transition hover:bg-emerald-500"
            @click="createAndPublish"
          >
            {{ t('adminAnnounce.publishNow') }}
          </button>
        </div>
      </div>
    </section>

    <section class="min-h-0 flex-1 rounded-xl border border-white/[0.08] bg-white/[0.03] p-5">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 class="text-sm font-medium text-slate-200">{{ t('adminAnnounce.list') }}</h2>
        <span class="text-xs text-slate-500">{{ t('adminAnnounce.total', { n: total }) }}</span>
      </div>
      <ThTableSkeleton v-if="loadingUi && rows.length === 0" :rows="8" :columns="6" class="mb-4" />
      <div v-if="loadingUi && rows.length === 0" class="py-8 lg:hidden">
        <ThLoadingState />
      </div>
      <div class="relative hidden overflow-x-auto lg:block">
        <ThLoadingState v-if="loadingUi && rows.length > 0" variant="overlay" />
        <table class="w-full min-w-[640px] text-left text-sm">
          <thead>
            <tr class="border-b border-white/[0.06] text-xs uppercase tracking-wide text-slate-500">
              <th class="py-2 pr-3">{{ t('adminAnnounce.thId') }}</th>
              <th class="py-2 pr-3">{{ t('adminAnnounce.thTitle') }}</th>
              <th class="py-2 pr-3">{{ t('adminAnnounce.thAudience') }}</th>
              <th class="py-2 pr-3">{{ t('adminAnnounce.thStatus') }}</th>
              <th class="py-2 pr-3">{{ t('adminAnnounce.thPublishedAt') }}</th>
              <th class="py-2 text-right">{{ t('adminAnnounce.thActions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in rows" :key="r.id" class="border-b border-white/[0.04] text-slate-300">
              <td class="py-2.5 pr-3 font-mono text-xs text-slate-500">{{ r.id }}</td>
              <td class="max-w-[220px] truncate py-2.5 pr-3" :title="r.title">{{ r.title }}</td>
              <td class="py-2.5 pr-3 text-xs text-slate-500">
                <span v-if="r.audience_user_id != null && r.audience_user_id > 0">{{ t('adminAnnounce.audienceUser', { id: r.audience_user_id }) }}</span>
                <span v-else>{{ t('adminAnnounce.audienceAll') }}</span>
              </td>
              <td class="py-2.5 pr-3">
                <span v-if="r.published" class="text-emerald-300/90">{{ t('adminAnnounce.statusOn') }}</span>
                <span v-else class="text-slate-500">{{ t('adminAnnounce.statusDraft') }}</span>
              </td>
              <td class="py-2.5 pr-3 text-xs text-slate-500">{{ r.published_at || '—' }}</td>
              <td class="py-2.5 text-right">
                <button type="button" class="mr-1 text-xs text-amber-200/90 hover:underline" @click="openEdit(r)">{{ t('common.edit') }}</button>
                <button v-if="!r.published" type="button" class="mr-1 text-xs text-emerald-300/90 hover:underline" @click="publish(r.id)">
                  {{ t('adminAnnounce.publish') }}
                </button>
                <button v-else type="button" class="mr-1 text-xs text-slate-400 hover:underline" @click="unpublish(r.id)">
                  {{ t('adminAnnounce.unpublish') }}
                </button>
                <button type="button" class="text-xs text-rose-300/90 hover:underline" @click="remove(r.id)">{{ t('common.delete') }}</button>
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
            <span v-if="r.published" class="text-emerald-300/90">{{ t('adminAnnounce.statusOn') }}</span>
            <span v-else class="text-slate-500">{{ t('adminAnnounce.statusDraft') }}</span>
          </MobileLabelValueRow>
          <MobileLabelValueRow :label="t('adminAnnounce.thPublishedAt')">{{ r.published_at || '—' }}</MobileLabelValueRow>
          <div class="flex flex-wrap gap-2 pt-1">
            <button type="button" class="text-xs text-amber-200/90 hover:underline" @click="openEdit(r)">{{ t('common.edit') }}</button>
            <button v-if="!r.published" type="button" class="text-xs text-emerald-300/90 hover:underline" @click="publish(r.id)">{{ t('adminAnnounce.publish') }}</button>
            <button v-else type="button" class="text-xs text-slate-400 hover:underline" @click="unpublish(r.id)">{{ t('adminAnnounce.unpublish') }}</button>
            <button type="button" class="text-xs text-rose-300/90 hover:underline" @click="remove(r.id)">{{ t('common.delete') }}</button>
          </div>
        </MobileLabelValueCard>
      </div>
      <div class="mt-4 flex items-center justify-end gap-2 text-xs">
        <button
          type="button"
          class="rounded-lg border border-white/[0.1] px-3 py-1.5 text-slate-400 transition hover:bg-white/[0.06]"
          :disabled="page <= 1"
          @click="prevPage"
        >
          {{ t('adminCommon.prevPage') }}
        </button>
        <button
          type="button"
          class="rounded-lg border border-white/[0.1] px-3 py-1.5 text-slate-400 transition hover:bg-white/[0.06]"
          :disabled="page * pageSize >= total"
          @click="nextPage"
        >
          {{ t('adminCommon.nextPage') }}
        </button>
      </div>
    </section>

    <Teleport to="body">
      <div
        v-if="editOpen"
        class="fixed inset-0 z-[100] flex items-center justify-center bg-black/60 p-4"
        role="dialog"
        @click.self="editOpen = false"
      >
        <div class="w-full max-w-lg rounded-xl border border-white/[0.1] bg-slate-950 p-5 shadow-2xl">
          <h3 class="text-sm font-semibold text-slate-100">{{ t('adminAnnounce.editTitle', { id: editForm.id }) }}</h3>
          <div class="mt-4 flex flex-col gap-3">
            <input
              v-model="editForm.title"
              type="text"
              maxlength="500"
              class="w-full rounded-lg border border-white/[0.1] bg-slate-900 px-3 py-2 text-sm text-slate-100 outline-none focus:border-amber-500/40"
            />
            <textarea v-model="editForm.content" rows="6" class="w-full resize-y rounded-lg border border-white/[0.1] bg-slate-900 px-3 py-2 text-sm text-slate-100 outline-none focus:border-amber-500/40" />
          </div>
          <div class="mt-5 flex justify-end gap-2">
            <button type="button" class="rounded-lg px-3 py-1.5 text-sm text-slate-400 hover:bg-white/[0.06]" @click="editOpen = false">
              {{ t('adminCommon.cancel') }}
            </button>
            <button type="button" class="rounded-lg bg-amber-600/90 px-4 py-1.5 text-sm font-medium text-amber-50 hover:bg-amber-500" @click="saveEdit">
              {{ t('adminCommon.save') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
