<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import * as pgw from '../api/playgroundGateway'

const { t } = useI18n()

const LS_KEY = 'tracehub_playground_settings_v1'
const SS_KEY = 'tracehub_playground_apikey'
const LS_MSG = 'tracehub_playground_messages_v1'

type ChatMsg = { id: string; role: 'user' | 'assistant'; content: string; imageUrl?: string }

const apiKey = ref('')
const stickySession = ref('')
const models = ref<string[]>([])
const model = ref('')
const modelsErr = ref('')
const modelsLoading = ref(false)

const systemPrompt = ref('')
const customBody = ref(false)
const customBodyJson = ref('{\n  "model": "gpt-4o-mini",\n  "messages": []\n}')

const useStream = ref(true)
const useImage = ref(false)
const imageUrl = ref('')

const useTemp = ref(true)
const temperature = ref(0.7)
const useTopP = ref(true)
const topP = ref(1)
const useFreqPen = ref(false)
const freqPen = ref(0)
const usePresPen = ref(false)
const presPen = ref(0)
const useMaxTokens = ref(false)
const maxTokens = ref(2048)
const useSeed = ref(false)
const seed = ref(0)

const messages = ref<ChatMsg[]>([])
const input = ref('')
const sending = ref(false)
const abortCtrl = ref<AbortController | null>(null)

const showDebug = ref(false)
const lastRequest = ref('')
const lastResponse = ref('')
const streamLog = ref('')

const editingId = ref<string | null>(null)
const editDraft = ref('')

function uid() {
  return crypto.randomUUID()
}

function parseErr(text: string) {
  try {
    return (JSON.parse(text) as { error?: { message?: string } }).error?.message || text
  } catch {
    return text
  }
}

onMounted(() => {
  stickySession.value = uid()
  try {
    apiKey.value = sessionStorage.getItem(SS_KEY) || ''
  } catch {
    /* ignore */
  }
  loadSettings()
  try {
    const raw = localStorage.getItem(LS_MSG)
    if (raw) {
      const arr = JSON.parse(raw) as ChatMsg[]
      if (Array.isArray(arr)) messages.value = arr
    }
  } catch {
    /* ignore */
  }
  if (apiKey.value) void refreshModels()
})

watch(apiKey, (v) => {
  try {
    if (v.trim()) sessionStorage.setItem(SS_KEY, v.trim())
    else sessionStorage.removeItem(SS_KEY)
  } catch {
    /* ignore */
  }
})

watch(messages, persistConversation, { deep: true })

function loadSettings() {
  try {
    const raw = localStorage.getItem(LS_KEY)
    if (!raw) return
    const j = JSON.parse(raw) as Record<string, unknown>
    if (typeof j.systemPrompt === 'string') systemPrompt.value = j.systemPrompt
    if (typeof j.model === 'string') model.value = j.model
    if (typeof j.temperature === 'number') temperature.value = j.temperature
    if (typeof j.topP === 'number') topP.value = j.topP
    if (typeof j.freqPen === 'number') freqPen.value = j.freqPen
    if (typeof j.presPen === 'number') presPen.value = j.presPen
    if (typeof j.maxTokens === 'number') maxTokens.value = j.maxTokens
    if (typeof j.useTemp === 'boolean') useTemp.value = j.useTemp
    if (typeof j.useTopP === 'boolean') useTopP.value = j.useTopP
    if (typeof j.useFreqPen === 'boolean') useFreqPen.value = j.useFreqPen
    if (typeof j.usePresPen === 'boolean') usePresPen.value = j.usePresPen
    if (typeof j.useMaxTokens === 'boolean') useMaxTokens.value = j.useMaxTokens
    if (typeof j.useStream === 'boolean') useStream.value = j.useStream
    if (typeof j.useImage === 'boolean') useImage.value = j.useImage
    if (typeof j.useSeed === 'boolean') useSeed.value = j.useSeed
    if (typeof j.seed === 'number') seed.value = j.seed
  } catch {
    /* ignore */
  }
}

function persistSettings() {
  const payload = {
    systemPrompt: systemPrompt.value,
    model: model.value,
    temperature: temperature.value,
    topP: topP.value,
    freqPen: freqPen.value,
    presPen: presPen.value,
    maxTokens: maxTokens.value,
    useTemp: useTemp.value,
    useTopP: useTopP.value,
    useFreqPen: useFreqPen.value,
    usePresPen: usePresPen.value,
    useMaxTokens: useMaxTokens.value,
    useStream: useStream.value,
    useImage: useImage.value,
    useSeed: useSeed.value,
    seed: seed.value,
  }
  try {
    localStorage.setItem(LS_KEY, JSON.stringify(payload))
  } catch {
    /* ignore */
  }
}

watch(
  [
    systemPrompt,
    model,
    temperature,
    topP,
    freqPen,
    presPen,
    maxTokens,
    useTemp,
    useTopP,
    useFreqPen,
    usePresPen,
    useMaxTokens,
    useStream,
    useImage,
    useSeed,
    seed,
  ],
  () => persistSettings(),
  { deep: true },
)

async function refreshModels() {
  const k = apiKey.value.trim()
  if (!k) {
    modelsErr.value = t('playground.needKey')
    return
  }
  modelsErr.value = ''
  modelsLoading.value = true
  try {
    const ids = await pgw.fetchModelIds(k, stickySession.value)
    models.value = ids
    if (!model.value && ids.length) model.value = ids[0]
    if (model.value && !ids.includes(model.value) && ids.length) model.value = ids[0]
  } catch (e: unknown) {
    modelsErr.value = e instanceof Error ? e.message : t('playground.modelsFail')
  } finally {
    modelsLoading.value = false
  }
}

async function pasteKey() {
  try {
    const text = await navigator.clipboard.readText()
    if (text.trim()) apiKey.value = text.trim()
  } catch {
    /* ignore */
  }
}

function buildOpenAIMessagesForHistory(): Array<Record<string, unknown>> {
  const out: Array<Record<string, unknown>> = []
  if (systemPrompt.value.trim()) {
    out.push({ role: 'system', content: systemPrompt.value.trim() })
  }
  for (const m of messages.value) {
    if (m.role === 'user' && m.imageUrl?.trim()) {
      out.push({
        role: 'user',
        content: [
          { type: 'text', text: m.content },
          { type: 'image_url', image_url: { url: m.imageUrl.trim() } },
        ],
      })
    } else {
      out.push({ role: m.role, content: m.content })
    }
  }
  return out
}

function buildNormalBody(appendUser: { text: string; img?: string }): Record<string, unknown> {
  const history = buildOpenAIMessagesForHistory()
  let userMsg: Record<string, unknown>
  if (appendUser.img?.trim()) {
    userMsg = {
      role: 'user',
      content: [
        { type: 'text', text: appendUser.text },
        { type: 'image_url', image_url: { url: appendUser.img.trim() } },
      ],
    }
  } else {
    userMsg = { role: 'user', content: appendUser.text }
  }
  const body: Record<string, unknown> = {
    model: model.value.trim(),
    messages: [...history, userMsg],
  }
  if (useTemp.value) body.temperature = temperature.value
  if (useTopP.value) body.top_p = topP.value
  if (useFreqPen.value) body.frequency_penalty = freqPen.value
  if (usePresPen.value) body.presence_penalty = presPen.value
  if (useMaxTokens.value && maxTokens.value > 0) body.max_tokens = maxTokens.value
  if (useSeed.value) body.seed = Math.floor(seed.value)
  return body
}

function buildCustomBody(text: string, img?: string): Record<string, unknown> {
  const j = JSON.parse(customBodyJson.value) as Record<string, unknown>
  const msgs = j.messages
  if (Array.isArray(msgs) && text) {
    if (img?.trim()) {
      return {
        ...j,
        messages: [
          ...msgs,
          {
            role: 'user',
            content: [
              { type: 'text', text },
              { type: 'image_url', image_url: { url: img.trim() } },
            ],
          },
        ],
      }
    }
    return { ...j, messages: [...msgs, { role: 'user', content: text }] }
  }
  return { ...j }
}

async function sendMessage() {
  const text = input.value.trim()
  if (!text || sending.value) return
  const k = apiKey.value.trim()
  if (!k) {
    modelsErr.value = t('playground.needKey')
    return
  }
  if (!customBody.value && !model.value.trim()) {
    modelsErr.value = t('playground.needModel')
    return
  }

  let body: Record<string, unknown>
  try {
    if (customBody.value) {
      body = buildCustomBody(text, useImage.value ? imageUrl.value : undefined)
    } else {
      body = buildNormalBody({
        text,
        img: useImage.value && imageUrl.value.trim() ? imageUrl.value : undefined,
      })
    }
  } catch {
    modelsErr.value = t('playground.badCustomJson')
    return
  }

  const imgSnap = useImage.value && imageUrl.value.trim() ? imageUrl.value.trim() : undefined
  messages.value = [
    ...messages.value,
    { id: uid(), role: 'user', content: text, imageUrl: imgSnap },
  ]
  input.value = ''
  imageUrl.value = ''

  const assistantId = uid()
  messages.value = [...messages.value, { id: assistantId, role: 'assistant', content: '' }]

  lastRequest.value = JSON.stringify({ ...body, stream: useStream.value }, null, 2)
  lastResponse.value = ''
  streamLog.value = ''
  modelsErr.value = ''
  sending.value = true

  try {
    if (useStream.value) {
      abortCtrl.value = new AbortController()
      const res = await fetch('/v1/chat/completions', {
        method: 'POST',
        signal: abortCtrl.value.signal,
        headers: {
          Authorization: `Bearer ${k}`,
          'Content-Type': 'application/json',
          'X-TraceHub-Session': stickySession.value,
        },
        body: JSON.stringify({ ...body, stream: true }),
      })
      if (!res.ok) {
        const errText = await res.text()
        throw new Error(parseErr(errText))
      }
      const reader = res.body?.getReader()
      if (!reader) throw new Error('No body')
      const dec = new TextDecoder()
      let buf = ''
      let acc = ''
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buf += dec.decode(value, { stream: true })
        const lines = buf.split('\n')
        buf = lines.pop() || ''
        for (const line of lines) {
          streamLog.value += `${line}\n`
          const s = line.trim()
          if (!s.startsWith('data:')) continue
          const data = s.slice(5).trim()
          if (data === '[DONE]') continue
          try {
            const json = JSON.parse(data) as { choices?: Array<{ delta?: { content?: string } }> }
            const piece = json.choices?.[0]?.delta?.content
            if (piece) {
              acc += piece
              patchAssistant(assistantId, acc)
            }
          } catch {
            /* ignore */
          }
        }
      }
      lastResponse.value = acc || streamLog.value.slice(0, 8000)
    } else {
      const r = await pgw.chatCompletion(k, stickySession.value, body)
      lastResponse.value = r.raw
      patchAssistant(assistantId, r.content)
    }
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e)
    modelsErr.value = msg
    patchAssistant(assistantId, `【${t('playground.error')}】\n\n${msg}`)
  } finally {
    sending.value = false
    abortCtrl.value = null
    if (customBody.value) {
      try {
        customBodyJson.value = JSON.stringify(body, null, 2)
      } catch {
        /* ignore */
      }
    }
  }
}

function patchAssistant(id: string, content: string) {
  messages.value = messages.value.map((m) => (m.id === id ? { ...m, content } : m))
}

function stopStream() {
  abortCtrl.value?.abort()
}

function clearChat() {
  messages.value = []
  lastResponse.value = ''
  streamLog.value = ''
  try {
    localStorage.removeItem(LS_MSG)
  } catch {
    /* ignore */
  }
}

function persistConversation() {
  try {
    localStorage.setItem(LS_MSG, JSON.stringify(messages.value))
  } catch {
    /* ignore */
  }
}

function copyText(s: string) {
  void navigator.clipboard.writeText(s)
}

function removeMessage(id: string) {
  messages.value = messages.value.filter((m) => m.id !== id)
}

function startEdit(m: ChatMsg) {
  editingId.value = m.id
  editDraft.value = m.content
}

function saveEdit() {
  if (!editingId.value) return
  messages.value = messages.value.map((m) =>
    m.id === editingId.value ? { ...m, content: editDraft.value } : m,
  )
  editingId.value = null
}

function cancelEdit() {
  editingId.value = null
}

function switchToUser(id: string) {
  messages.value = messages.value.map((m) => (m.id === id ? { ...m, role: 'user' } : m))
}

async function regenerate() {
  const lastAi = [...messages.value].map((m, i) => ({ m, i })).filter((x) => x.m.role === 'assistant').pop()
  if (!lastAi) return
  const before = messages.value.slice(0, lastAi.i)
  const lastUser = before[before.length - 1]
  if (!lastUser || lastUser.role !== 'user') return
  messages.value = before.slice(0, -1)
  input.value = lastUser.content
  if (lastUser.imageUrl) {
    useImage.value = true
    imageUrl.value = lastUser.imageUrl
  } else {
    imageUrl.value = ''
  }
  await sendMessage()
}

function exportSettings() {
  const blob = new Blob(
    [
      JSON.stringify(
        {
          version: 1,
          systemPrompt: systemPrompt.value,
          model: model.value,
          params: {
            temperature: useTemp.value ? temperature.value : null,
            top_p: useTopP.value ? topP.value : null,
            frequency_penalty: useFreqPen.value ? freqPen.value : null,
            presence_penalty: usePresPen.value ? presPen.value : null,
            max_tokens: useMaxTokens.value ? maxTokens.value : null,
            seed: useSeed.value ? seed.value : null,
          },
          flags: {
            useStream: useStream.value,
            useImage: useImage.value,
            customBody: customBody.value,
          },
        },
        null,
        2,
      ),
    ],
    { type: 'application/json' },
  )
  downloadBlob(blob, 'tracehub-playground-settings.json')
}

function importSettings(ev: Event) {
  const inputEl = ev.target as HTMLInputElement
  const file = inputEl.files?.[0]
  inputEl.value = ''
  if (!file) return
  const reader = new FileReader()
  reader.onload = () => {
    try {
      const j = JSON.parse(String(reader.result)) as {
        systemPrompt?: string
        model?: string
        params?: Record<string, number | null>
        flags?: Record<string, boolean>
      }
      if (typeof j.systemPrompt === 'string') systemPrompt.value = j.systemPrompt
      if (typeof j.model === 'string') model.value = j.model
      const p = j.params || {}
      if (p.temperature != null) {
        useTemp.value = true
        temperature.value = p.temperature
      }
      if (p.top_p != null) {
        useTopP.value = true
        topP.value = p.top_p
      }
      if (p.frequency_penalty != null) {
        useFreqPen.value = true
        freqPen.value = p.frequency_penalty
      }
      if (p.presence_penalty != null) {
        usePresPen.value = true
        presPen.value = p.presence_penalty
      }
      if (p.max_tokens != null) {
        useMaxTokens.value = true
        maxTokens.value = Math.floor(p.max_tokens)
      }
      if (p.seed != null) {
        useSeed.value = true
        seed.value = Math.floor(p.seed)
      }
      const f = j.flags || {}
      if (typeof f.useStream === 'boolean') useStream.value = f.useStream
      if (typeof f.useImage === 'boolean') useImage.value = f.useImage
      if (typeof f.customBody === 'boolean') customBody.value = f.customBody
      persistSettings()
    } catch {
      modelsErr.value = t('playground.importFail')
    }
  }
  reader.readAsText(file)
}

function exportConversation() {
  const blob = new Blob([JSON.stringify({ version: 1, messages: messages.value }, null, 2)], {
    type: 'application/json',
  })
  downloadBlob(blob, 'tracehub-playground-chat.json')
}

function downloadBlob(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = name
  a.click()
  URL.revokeObjectURL(url)
}

const canSend = computed(() => apiKey.value.trim().length > 0 && !sending.value)

const chatScrollEl = ref<HTMLElement | null>(null)

function scrollChatToBottom() {
  nextTick(() => {
    const el = chatScrollEl.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

watch(
  () => [messages.value.length, messages.value.map((m) => m.content).join('\x00'), sending.value],
  () => scrollChatToBottom(),
)

function fmtNum(n: number) {
  return Number.isInteger(n) ? String(n) : String(n)
}
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col gap-4 lg:flex-row lg:gap-0 lg:items-stretch">
    <section
      class="flex max-h-[min(52vh,26rem)] w-full shrink-0 flex-col overflow-hidden rounded-2xl border border-white/[0.08] bg-slate-950/40 lg:max-h-none lg:max-w-[22rem] lg:self-stretch lg:rounded-r-none lg:border-r lg:border-white/[0.06]"
    >
      <div class="shrink-0 border-b border-white/[0.06] px-4 py-3">
        <p class="text-[11px] font-semibold uppercase tracking-wider text-slate-500">{{ t('playground.configKicker') }}</p>
        <h1 class="text-lg font-semibold text-slate-100">{{ t('playground.title') }}</h1>
        <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('playground.desc') }}</p>
      </div>

      <div class="thin-scrollbar min-h-0 flex-1 space-y-4 overflow-y-auto overscroll-y-contain px-4 py-4">
        <div class="rounded-xl border border-white/[0.06] bg-black/30 p-3">
          <label class="flex cursor-pointer items-center justify-between gap-2 text-sm text-slate-300">
            <span>{{ t('playground.customBody') }}</span>
            <input v-model="customBody" type="checkbox" class="h-4 w-4 rounded border-slate-600 text-cyan-500" />
          </label>
          <p class="mt-1 text-[11px] text-slate-600">{{ t('playground.customBodyHint') }}</p>
          <textarea
            v-if="customBody"
            v-model="customBodyJson"
            rows="8"
            class="mt-2 w-full rounded-lg border border-slate-700 bg-slate-950 px-2 py-1.5 font-mono text-[11px] text-slate-200"
          />
        </div>

        <div>
          <label class="text-xs text-slate-500">{{ t('playground.apiKey') }}</label>
          <div class="mt-1 flex gap-2">
            <input
              v-model="apiKey"
              type="password"
              autocomplete="off"
              :placeholder="t('playground.apiKeyPh')"
              class="min-w-0 flex-1 rounded-lg border border-slate-600 bg-slate-950 px-2 py-1.5 font-mono text-xs"
            />
            <button type="button" class="shrink-0 rounded-lg border border-slate-600 px-2 py-1 text-xs" @click="pasteKey">
              {{ t('playground.paste') }}
            </button>
          </div>
          <RouterLink to="/console/keys" class="mt-1 inline-block text-[11px] text-cyan-400 hover:underline">
            {{ t('playground.goKeys') }}
          </RouterLink>
        </div>

        <div class="flex flex-wrap gap-2">
          <button
            type="button"
            class="rounded-lg bg-cyan-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-cyan-500 disabled:opacity-50"
            :disabled="modelsLoading"
            @click="refreshModels"
          >
            {{ modelsLoading ? t('common.loading') : t('playground.refreshModels') }}
          </button>
          <label class="flex cursor-pointer items-center gap-2 text-xs text-slate-400">
            <input v-model="useStream" type="checkbox" class="h-3.5 w-3.5 rounded border-slate-600 text-cyan-500" />
            {{ t('playground.stream') }}
          </label>
        </div>
        <p v-if="modelsErr" class="text-xs text-red-400">{{ modelsErr }}</p>

        <template v-if="!customBody">
          <div>
            <label class="text-xs text-slate-500">{{ t('playground.model') }}</label>
            <select v-model="model" class="mt-1 w-full rounded-lg border border-slate-600 bg-slate-950 px-2 py-2 text-sm">
              <option v-if="!models.length" value="">{{ t('playground.modelEmpty') }}</option>
              <option v-for="m in models" :key="m" :value="m">{{ m }}</option>
            </select>
          </div>

          <div>
            <label class="text-xs text-slate-500">{{ t('playground.system') }}</label>
            <textarea
              v-model="systemPrompt"
              rows="3"
              class="mt-1 w-full rounded-lg border border-slate-600 bg-slate-950 px-2 py-1.5 text-xs"
              :placeholder="t('playground.systemPh')"
            />
          </div>

          <div class="rounded-xl border border-white/[0.06] bg-black/25 p-3">
            <label class="flex cursor-pointer items-center justify-between text-sm text-slate-300">
              <span>{{ t('playground.imageUrl') }}</span>
              <input v-model="useImage" type="checkbox" class="h-4 w-4 rounded border-slate-600 text-cyan-500" />
            </label>
            <input
              v-if="useImage"
              v-model="imageUrl"
              type="url"
              class="mt-2 w-full rounded-lg border border-slate-600 bg-slate-950 px-2 py-1.5 text-xs"
              :placeholder="t('playground.imagePh')"
            />
            <p class="mt-1 text-[11px] text-slate-600">{{ t('playground.imageHint') }}</p>
          </div>

          <div class="space-y-3 rounded-xl border border-white/[0.06] bg-black/25 p-3">
            <div class="space-y-1">
              <div class="flex items-center justify-between gap-2">
                <label class="flex cursor-pointer items-center gap-2 text-xs text-slate-400">
                  <input v-model="useTemp" type="checkbox" class="h-3.5 w-3.5 rounded border-slate-600 text-cyan-500" />
                  <span>{{ t('playground.temp') }}</span>
                </label>
                <span class="font-mono text-[11px] text-cyan-200/90">{{ fmtNum(temperature) }}</span>
              </div>
              <input
                v-model.number="temperature"
                type="range"
                min="0"
                max="2"
                step="0.05"
                :disabled="!useTemp"
                class="w-full accent-cyan-500 disabled:opacity-40"
              />
            </div>
            <div class="space-y-1">
              <div class="flex items-center justify-between gap-2">
                <label class="flex cursor-pointer items-center gap-2 text-xs text-slate-400">
                  <input v-model="useTopP" type="checkbox" class="h-3.5 w-3.5 rounded border-slate-600 text-cyan-500" />
                  <span>{{ t('playground.topP') }}</span>
                </label>
                <span class="font-mono text-[11px] text-cyan-200/90">{{ fmtNum(topP) }}</span>
              </div>
              <input
                v-model.number="topP"
                type="range"
                min="0"
                max="1"
                step="0.01"
                :disabled="!useTopP"
                class="w-full accent-cyan-500 disabled:opacity-40"
              />
            </div>
            <div class="space-y-1">
              <div class="flex items-center justify-between gap-2">
                <label class="flex cursor-pointer items-center gap-2 text-xs text-slate-400">
                  <input v-model="useFreqPen" type="checkbox" class="h-3.5 w-3.5 rounded border-slate-600 text-cyan-500" />
                  <span>{{ t('playground.freqPen') }}</span>
                </label>
                <span class="font-mono text-[11px] text-cyan-200/90">{{ fmtNum(freqPen) }}</span>
              </div>
              <input
                v-model.number="freqPen"
                type="range"
                min="-2"
                max="2"
                step="0.1"
                :disabled="!useFreqPen"
                class="w-full accent-cyan-500 disabled:opacity-40"
              />
            </div>
            <div class="space-y-1">
              <div class="flex items-center justify-between gap-2">
                <label class="flex cursor-pointer items-center gap-2 text-xs text-slate-400">
                  <input v-model="usePresPen" type="checkbox" class="h-3.5 w-3.5 rounded border-slate-600 text-cyan-500" />
                  <span>{{ t('playground.presPen') }}</span>
                </label>
                <span class="font-mono text-[11px] text-cyan-200/90">{{ fmtNum(presPen) }}</span>
              </div>
              <input
                v-model.number="presPen"
                type="range"
                min="-2"
                max="2"
                step="0.1"
                :disabled="!usePresPen"
                class="w-full accent-cyan-500 disabled:opacity-40"
              />
            </div>
          </div>

          <div class="flex flex-wrap items-center gap-3 rounded-xl border border-white/[0.06] bg-black/25 px-3 py-2">
            <label class="flex cursor-pointer items-center gap-2 text-xs text-slate-400">
              <input v-model="useMaxTokens" type="checkbox" class="h-3.5 w-3.5 rounded border-slate-600 text-cyan-500" />
              {{ t('playground.maxTokens') }}
            </label>
            <input
              v-model.number="maxTokens"
              type="number"
              min="1"
              class="w-24 rounded border border-slate-600 bg-slate-950 px-2 py-1 font-mono text-xs"
            />
          </div>

          <div class="flex flex-wrap items-center gap-3 rounded-xl border border-white/[0.06] bg-black/25 px-3 py-2">
            <label class="flex cursor-pointer items-center gap-2 text-xs text-slate-400">
              <input v-model="useSeed" type="checkbox" class="h-3.5 w-3.5 rounded border-slate-600 text-cyan-500" />
              {{ t('playground.seed') }}
            </label>
            <input v-model.number="seed" type="number" class="w-28 rounded border border-slate-600 bg-slate-950 px-2 py-1 font-mono text-xs" />
          </div>
        </template>

        <div v-else class="rounded-xl border border-amber-500/20 bg-amber-950/20 p-3 text-xs text-amber-200/90">
          {{ t('playground.customBodyWarn') }}
        </div>

        <div class="flex flex-wrap gap-2 border-t border-white/[0.06] pt-3">
          <button type="button" class="rounded-lg bg-cyan-600 px-3 py-1.5 text-xs font-medium text-white" @click="exportSettings">
            {{ t('playground.export') }}
          </button>
          <label class="cursor-pointer rounded-lg border border-slate-600 px-3 py-1.5 text-xs text-slate-300 hover:bg-white/5">
            {{ t('playground.import') }}
            <input type="file" accept="application/json" class="hidden" @change="importSettings" />
          </label>
        </div>
      </div>
    </section>

    <section
      class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden rounded-2xl border border-white/[0.08] bg-slate-950/30 lg:min-h-0 lg:rounded-l-none lg:border-l-0"
    >
      <header class="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b border-white/[0.06] px-4 py-3">
        <div>
          <p class="text-sm font-semibold text-slate-100">{{ t('playground.chatTitle') }}</p>
          <p class="text-[11px] text-slate-500">
            {{ model || '—' }} · {{ useStream ? t('playground.streamOn') : t('playground.streamOff') }}
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button
            type="button"
            class="rounded-lg border border-slate-600 px-3 py-1.5 text-xs text-slate-300 hover:bg-white/5"
            @click="showDebug = !showDebug"
          >
            {{ showDebug ? t('playground.hideDebug') : t('playground.showDebug') }}
          </button>
          <button type="button" class="rounded-lg border border-slate-600 px-3 py-1.5 text-xs text-slate-300 hover:bg-white/5" @click="exportConversation">
            {{ t('playground.exportChat') }}
          </button>
        </div>
      </header>

      <div v-if="showDebug" class="shrink-0 border-b border-white/[0.06] bg-black/40 px-4 py-2">
        <p class="text-[10px] font-medium uppercase text-slate-500">{{ t('playground.debugReq') }}</p>
        <pre class="mt-1 max-h-32 overflow-auto font-mono text-[10px] text-slate-400">{{ lastRequest || '—' }}</pre>
        <p class="mt-2 text-[10px] font-medium uppercase text-slate-500">{{ t('playground.debugRes') }}</p>
        <pre class="mt-1 max-h-32 overflow-auto font-mono text-[10px] text-slate-400">{{ lastResponse || streamLog.slice(0, 4000) || '—' }}</pre>
      </div>

      <div
        ref="chatScrollEl"
        class="thin-scrollbar min-h-0 flex-1 overflow-y-auto overscroll-y-contain px-4 py-4"
      >
        <div class="flex flex-col gap-3">
        <div v-if="!messages.length" class="flex flex-1 flex-col items-center justify-center py-16 text-center text-sm text-slate-500">
          <p>{{ t('playground.emptyChat') }}</p>
        </div>
        <div
          v-for="m in messages"
          :key="m.id"
          class="flex"
          :class="m.role === 'user' ? 'justify-end' : 'justify-start'"
        >
          <div
            class="max-w-[min(100%,42rem)] rounded-2xl border px-4 py-2.5 text-sm leading-relaxed"
            :class="
              m.role === 'user'
                ? 'border-cyan-500/25 bg-cyan-950/35 text-slate-100'
                : 'border-white/[0.08] bg-slate-900/80 text-slate-200'
            "
          >
            <div v-if="m.imageUrl" class="mb-2">
              <img
                :src="m.imageUrl"
                alt=""
                class="max-h-40 max-w-full rounded-lg border border-white/10 object-contain"
                @error="(($event.target as HTMLImageElement).style.display = 'none')"
              />
            </div>
            <div v-if="editingId === m.id" class="space-y-2">
              <textarea v-model="editDraft" rows="4" class="w-full rounded border border-slate-600 bg-slate-950 px-2 py-1 text-xs" />
              <div class="flex gap-2">
                <button type="button" class="text-xs text-cyan-400 hover:underline" @click="saveEdit">{{ t('common.save') }}</button>
                <button type="button" class="text-xs text-slate-500 hover:underline" @click="cancelEdit">{{ t('common.cancel') }}</button>
              </div>
            </div>
            <div v-else class="whitespace-pre-wrap break-words">{{ m.content || (sending && m.role === 'assistant' ? '…' : '') }}</div>
            <div v-if="editingId !== m.id" class="mt-2 flex flex-wrap gap-2 border-t border-white/[0.06] pt-2 text-[11px]">
              <button type="button" class="text-slate-500 hover:text-cyan-400" @click="copyText(m.content)">{{ t('playground.copy') }}</button>
              <button type="button" class="text-slate-500 hover:text-cyan-400" @click="startEdit(m)">{{ t('common.edit') }}</button>
              <button type="button" class="text-slate-500 hover:text-red-400" @click="removeMessage(m.id)">{{ t('common.delete') }}</button>
              <button v-if="m.role === 'assistant'" type="button" class="text-slate-500 hover:text-amber-400" @click="regenerate">
                {{ t('playground.regen') }}
              </button>
              <button v-if="m.role === 'assistant'" type="button" class="text-slate-500 hover:text-violet-400" @click="switchToUser(m.id)">
                {{ t('playground.asUser') }}
              </button>
            </div>
          </div>
        </div>
        </div>
      </div>

      <footer class="shrink-0 border-t border-white/[0.06] bg-slate-950/95 p-4 shadow-[0_-12px_32px_-8px_rgba(0,0,0,0.45)] backdrop-blur-sm">
        <div class="flex gap-2">
          <button
            type="button"
            class="rounded-xl border border-slate-600 p-2 text-slate-400 hover:bg-red-950/40 hover:text-red-300"
            :title="t('playground.clear')"
            @click="clearChat"
          >
            <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0"
              />
            </svg>
          </button>
          <textarea
            v-model="input"
            rows="2"
            class="min-h-[3rem] flex-1 resize-y rounded-xl border border-slate-600 bg-slate-950 px-3 py-2 text-sm text-slate-100 placeholder:text-slate-600"
            :placeholder="t('playground.inputPh')"
            @keydown.enter.exact.prevent="sendMessage"
          />
          <div class="flex flex-col gap-1">
            <button
              v-if="sending && useStream"
              type="button"
              class="rounded-xl border border-amber-500/40 bg-amber-950/40 px-3 py-2 text-xs font-medium text-amber-200"
              @click="stopStream"
            >
              {{ t('playground.stop') }}
            </button>
            <button
              type="button"
              :disabled="!canSend || !input.trim()"
              class="rounded-xl bg-cyan-600 px-4 py-2 text-sm font-semibold text-slate-950 hover:bg-cyan-500 disabled:opacity-40"
              @click="sendMessage"
            >
              {{ sending ? t('playground.sending') : t('playground.send') }}
            </button>
          </div>
        </div>
      </footer>
    </section>
  </div>
</template>
