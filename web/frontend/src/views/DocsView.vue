<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useSiteMetaStore } from '../stores/siteMeta'
import UserAccountMenu from '../components/UserAccountMenu.vue'
import ContactCardDisclosure from '../components/ContactCardDisclosure.vue'

const { t } = useI18n()
const auth = useAuthStore()
const siteMeta = useSiteMetaStore()
const activeId = ref('doc-intro')
const mobileTocOpen = ref(false)

const docsChatCurlURL = computed(() => {
  const b = siteMeta.effectiveOpenaiApiBase.replace(/\/+$/, '')
  return `${b}/chat/completions`
})

const toc = computed(() => [
  { id: 'doc-intro', label: t('docs.tocIntro') },
  { id: 'doc-concepts', label: t('docs.tocConcepts') },
  { id: 'doc-quickstart', label: t('docs.tocQuick') },
  { id: 'doc-console', label: t('docs.tocConsole') },
  { id: 'doc-auth', label: t('docs.tocAuth') },
  { id: 'doc-gateway', label: t('docs.tocGateway') },
  { id: 'doc-volc-media', label: t('docs.tocVolc') },
  { id: 'doc-headers', label: t('docs.tocHeaders') },
  { id: 'doc-routing', label: t('docs.tocRouting') },
  { id: 'doc-user-api', label: t('docs.tocUserApi') },
  { id: 'doc-admin-api', label: t('docs.tocAdminApi') },
  { id: 'doc-ops', label: t('docs.tocOps') },
  { id: 'doc-errors', label: t('docs.tocErrors') },
  { id: 'doc-faq', label: t('docs.tocFaq') },
])

let observer: IntersectionObserver | null = null

function scrollToSection(id: string) {
  activeId.value = id
  mobileTocOpen.value = false
  const el = document.getElementById(id)
  el?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

onMounted(() => {
  void siteMeta.ensureLoaded()
  observer = new IntersectionObserver(
    (entries) => {
      const visible = entries.filter((e) => e.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)
      if (visible.length) {
        activeId.value = visible[0].target.id
      }
    },
    { root: null, rootMargin: '-72px 0px -55% 0px', threshold: [0, 0.01] },
  )
  toc.value.forEach(({ id }) => {
    const el = document.getElementById(id)
    if (el) observer?.observe(el)
  })
})

onUnmounted(() => {
  observer?.disconnect()
  observer = null
})
</script>

<template>
  <div class="th-page-shell min-h-screen text-slate-100">
    <header class="th-site-header docs-chrome-header sticky top-0 z-30 backdrop-blur-md">
      <div class="mx-auto flex max-w-6xl items-center justify-between gap-4 px-4 py-3.5 sm:px-5">
        <RouterLink to="/" class="flex min-w-0 items-center gap-3 transition hover:opacity-90">
          <div class="th-logo-mark th-logo-mark--sm th-logo-mark--image shrink-0">
            <img :src="siteMeta.displayIconUrl" :alt="siteMeta.displayName" width="36" height="36" decoding="async" />
          </div>
          <div class="min-w-0">
            <span class="th-sidebar-brand block truncate text-lg font-semibold tracking-tight text-slate-100">{{ siteMeta.displayName }}</span>
            <span class="text-[10px] font-medium uppercase tracking-[0.18em] text-slate-500">{{ t('docs.center') }}</span>
          </div>
        </RouterLink>
        <nav class="flex shrink-0 flex-wrap items-center justify-end gap-1 sm:gap-2">
          <RouterLink to="/" class="th-site-nav-link">{{ t('nav.home') }}</RouterLink>
          <RouterLink
            :to="auth.isLoggedIn ? '/console' : '/login'"
            class="th-site-nav-link"
          >
            {{ auth.isLoggedIn ? t('nav.console') : t('nav.login') }}
          </RouterLink>
          <UserAccountMenu v-if="auth.isLoggedIn" accent="cyan" />
        </nav>
      </div>
    </header>

    <div class="mx-auto grid max-w-6xl gap-8 px-4 pb-20 pt-8 lg:grid-cols-[13.5rem_minmax(0,1fr)] lg:gap-12 lg:px-5 lg:pt-10">
      <!-- 移动端目录 -->
      <div class="lg:hidden">
        <button
          type="button"
          class="docs-mobile-toc-btn flex w-full items-center justify-between rounded-xl px-4 py-3 text-left text-sm font-medium"
          :aria-expanded="mobileTocOpen"
          @click="mobileTocOpen = !mobileTocOpen"
        >
          <span>{{ t('docs.mobileToc') }}</span>
          <svg class="h-4 w-4 text-slate-500 transition" :class="{ 'rotate-180': mobileTocOpen }" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
            <path fill-rule="evenodd" d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.94a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z" clip-rule="evenodd" />
          </svg>
        </button>
        <nav v-show="mobileTocOpen" class="docs-mobile-nav mt-2 space-y-0.5 rounded-xl p-2">
          <button
            v-for="item in toc"
            :key="item.id"
            type="button"
            class="docs-toc-link block w-full rounded-lg px-3 py-2 text-left text-sm"
            :class="{ 'docs-toc-link--active': activeId === item.id }"
            @click="scrollToSection(item.id)"
          >
            {{ item.label }}
          </button>
        </nav>
      </div>

      <!-- 桌面端固定目录 -->
      <aside class="docs-aside hidden lg:block">
        <div class="docs-aside-inner sticky top-24 max-h-[calc(100dvh-7rem)] overflow-y-auto pr-1">
          <p class="mb-3 text-[10px] font-semibold uppercase tracking-[0.2em] text-slate-600">{{ t('docs.tocHeading') }}</p>
          <nav class="docs-toc-rail space-y-0.5 pl-3">
            <button
              v-for="item in toc"
              :key="item.id"
              type="button"
              class="docs-toc-link -ml-px block w-full border-l-2 border-transparent py-1.5 pl-3 text-left text-xs leading-snug transition"
              :class="{ 'docs-toc-link--active': activeId === item.id }"
              @click="scrollToSection(item.id)"
            >
              {{ item.label }}
            </button>
          </nav>
        </div>
      </aside>

      <main class="docs-main min-w-0">
        <p class="th-kicker mb-2"><span class="th-kicker-accent">{{ t('docs.kicker') }}</span></p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-100 md:text-4xl">{{ t('docs.pageTitle') }}</h1>
        <p class="mt-3 max-w-2xl text-sm leading-relaxed text-slate-500">
          {{ t('docs.pageSub') }}
        </p>

        <article class="docs-prose mt-10 space-y-12">
          <section id="doc-intro" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>{{ t('docs.introH2') }}</h2>
            <p>{{ t('docs.introP1') }}</p>
            <p>{{ t('docs.introP2') }}</p>
          </section>

          <section id="doc-concepts" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>{{ t('docs.conceptsH2') }}</h2>
            <dl class="docs-dl mt-4">
              <dt>{{ t('docs.cJwt') }}</dt>
              <dd>{{ t('docs.cJwtD') }}</dd>
              <dt>{{ t('docs.cKey') }}</dt>
              <dd>{{ t('docs.cKeyD') }}</dd>
              <dt>{{ t('docs.cGroup') }}</dt>
              <dd>{{ t('docs.cGroupD') }}</dd>
              <dt>{{ t('docs.cChan') }}</dt>
              <dd>{{ t('docs.cChanD') }}</dd>
              <dt>{{ t('docs.cBill') }}</dt>
              <dd>{{ t('docs.cBillD') }}</dd>
            </dl>
          </section>

          <section id="doc-quickstart" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>{{ t('docs.quickH2') }}</h2>
            <ol class="docs-ol mt-4">
              <li>{{ t('docs.q1') }}</li>
              <li>
                {{ t('docs.q2before') }}
                <RouterLink to="/register" class="th-link-accent">{{ t('nav.register') }}</RouterLink>
                {{ t('docs.q2after') }}
              </li>
              <li>
                <RouterLink to="/login" class="th-link-accent">{{ t('nav.login') }}</RouterLink>
                {{ t('docs.q3after') }}
              </li>
              <li>{{ t('docs.q4', { base: siteMeta.effectiveOpenaiApiBase }) }}</li>
            </ol>
            <p class="docs-callout mt-6">
              <strong class="text-slate-200">{{ t('docs.curlTitle') }}</strong>{{ t('docs.curlHint') }}
            </p>
            <pre class="docs-pre mt-3">curl -sS "{{ docsChatCurlURL }}" \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role":"user","content":"用一句话介绍 TraceHub"}],
    "stream": false
  }'</pre>
            <p class="mt-4 text-sm text-slate-500">{{ t('docs.streamNote') }}</p>
          </section>

          <section id="doc-console" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>{{ t('docs.consoleH2') }}</h2>
            <p>{{ t('docs.consoleIntro') }}</p>
            <ul class="docs-ul mt-4">
              <li><strong class="text-slate-200">{{ t('docs.cliOverview') }}</strong>：{{ t('docs.cliOverviewD') }}</li>
              <li><strong class="text-slate-200">{{ t('docs.cliModels') }}</strong>：{{ t('docs.cliModelsD') }}</li>
              <li><strong class="text-slate-200">{{ t('docs.cliKeys') }}</strong>：{{ t('docs.cliKeysD') }}</li>
              <li><strong class="text-slate-200">{{ t('docs.cliLogs') }}</strong>：{{ t('docs.cliLogsD') }}</li>
              <li><strong class="text-slate-200">{{ t('docs.cliGroup') }}</strong>：{{ t('docs.cliGroupD') }}</li>
              <li><strong class="text-slate-200">{{ t('docs.cliBill') }}</strong>：{{ t('docs.cliBillD') }}</li>
              <li><strong class="text-slate-200">{{ t('docs.cliSet') }}</strong>：{{ t('docs.cliSetD') }}</li>
            </ul>
            <p v-if="auth.role === 'admin' || auth.role === 'superadmin'" class="docs-admin-note mt-4 rounded-lg border px-4 py-3 text-sm">
              {{ t('docs.adminNoteA') }}<RouterLink to="/admin" class="th-link-accent font-medium underline-offset-2 hover:underline">{{ t('layout.navAdmin') }}</RouterLink>{{ t('docs.adminNoteB') }}
            </p>
          </section>

          <section id="doc-auth" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>{{ t('docs.authH2') }}</h2>
            <h3>{{ t('docs.authGwH3') }}</h3>
            <pre class="docs-pre mt-2">Authorization: Bearer &lt;API Key&gt;</pre>
            <p class="mt-3">{{ t('docs.authGwP') }}</p>
            <h3 class="mt-6">{{ t('docs.authJwtH3') }}</h3>
            <p>{{ t('docs.authJwtP') }}</p>
            <pre class="docs-pre mt-2">Authorization: Bearer &lt;access_token&gt;</pre>
            <h3 class="mt-6">{{ t('docs.corsH3') }}</h3>
            <p>{{ t('docs.corsP') }}</p>
            <h3 class="mt-6">{{ t('docs.bestH3') }}</h3>
            <ul class="docs-ul">
              <li>{{ t('docs.best1') }}</li>
              <li>{{ t('docs.best2') }}</li>
              <li>{{ t('docs.best3') }}</li>
            </ul>
          </section>

          <section id="doc-gateway" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>{{ t('docs.gwH2') }}</h2>
            <p>{{ t('docs.gwIntro') }}</p>

            <h3 class="mt-6">{{ t('docs.catChat') }}</h3>
            <ul class="docs-endpoint-list font-mono text-xs">
              <li>{{ t('docs.epChat1') }}</li>
              <li>{{ t('docs.epChat2') }}</li>
            </ul>

            <h3 class="mt-6">{{ t('docs.catEmb') }}</h3>
            <ul class="docs-endpoint-list font-mono text-xs">
              <li><span class="text-emerald-400/90">POST</span> /v1/embeddings</li>
              <li><span class="text-emerald-400/90">POST</span> /v1/moderations</li>
            </ul>

            <h3 class="mt-6">{{ t('docs.catImg') }}</h3>
            <ul class="docs-endpoint-list font-mono text-xs">
              <li><span class="text-emerald-400/90">POST</span> /v1/images/generations</li>
              <li><span class="text-emerald-400/90">POST</span> /v1/images/edits</li>
              <li><span class="text-emerald-400/90">POST</span> /v1/images/variations</li>
            </ul>

            <h3 class="mt-6">{{ t('docs.catVideo') }}</h3>
            <p class="text-sm text-slate-500">{{ t('docs.catVideoP') }}</p>
            <ul class="docs-endpoint-list mt-3 font-mono text-xs">
              <li>{{ t('docs.epVideo1') }}</li>
              <li>{{ t('docs.epVideo2') }}</li>
              <li>{{ t('docs.epVideo3') }}</li>
              <li>{{ t('docs.epVideo4') }}</li>
              <li>{{ t('docs.epVideo5') }}</li>
            </ul>

            <h3 class="mt-6">{{ t('docs.catAudio') }}</h3>
            <p class="text-sm text-slate-500">{{ t('docs.catAudioP') }}</p>
            <ul class="docs-endpoint-list mt-3 font-mono text-xs">
              <li>{{ t('docs.epAudio1') }}</li>
              <li>{{ t('docs.epAudio2') }}</li>
              <li>{{ t('docs.epAudio3') }}</li>
            </ul>

            <h3 class="mt-6">{{ t('docs.catModels') }}</h3>
            <ul class="docs-endpoint-list font-mono text-xs">
              <li><span class="text-sky-400/90">GET</span> /v1/models</li>
              <li><span class="text-sky-400/90">GET</span> /v1/models/:model_id</li>
            </ul>

            <h3 class="mt-6">{{ t('docs.catResp') }}</h3>
            <ul class="docs-endpoint-list font-mono text-xs">
              <li><span class="text-emerald-400/90">POST</span> /v1/responses</li>
              <li><span class="text-sky-400/90">GET</span> /v1/responses/:response_id</li>
              <li><span class="text-rose-400/90">DELETE</span> /v1/responses/:response_id</li>
              <li><span class="text-emerald-400/90">POST</span> /v1/responses/:response_id/cancel</li>
            </ul>

            <h3 class="mt-6">{{ t('docs.catRt') }}</h3>
            <p class="text-sm text-slate-500">{{ t('docs.catRtP') }}</p>

            <h3 class="mt-6">{{ t('docs.catFiles') }}</h3>
            <ul class="docs-endpoint-list font-mono text-xs">
              <li>{{ t('docs.epFiles1') }}</li>
              <li>{{ t('docs.epFiles2') }}</li>
              <li>{{ t('docs.epFiles3') }}</li>
            </ul>

            <h3 class="mt-6">{{ t('docs.catAssist') }}</h3>
            <p class="text-sm text-slate-400">{{ t('docs.assistP') }}</p>

            <h3 class="mt-6">{{ t('docs.passthroughH3') }}</h3>
            <p class="text-sm text-slate-400">{{ t('docs.passthroughP') }}</p>
          </section>

          <section id="doc-volc-media" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>{{ t('docs.tocVolc') }}</h2>
            <h3 class="mt-4">{{ t('docs.volcMediaH3') }}</h3>
            <p class="text-sm leading-relaxed text-slate-400">{{ t('docs.volcMediaP') }}</p>
            <ul class="mt-4 list-disc space-y-2 pl-5 text-sm text-slate-400">
              <li>{{ t('docs.volcSeedanceLi1') }}</li>
              <li>{{ t('docs.volcSeedanceLi2') }}</li>
              <li>{{ t('docs.volcSeed3dLi1') }}</li>
              <li>{{ t('docs.volcSeed3dLi2') }}</li>
              <li>{{ t('docs.volcSeed3dLi3') }}</li>
            </ul>
            <p class="mt-4 text-sm text-slate-500">{{ t('docs.volcFoot') }}</p>
          </section>

          <section id="doc-headers" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>{{ t('docs.hdrH2') }}</h2>
            <table class="docs-table mt-4">
              <thead>
                <tr>
                  <th>{{ t('docs.hdrThName') }}</th>
                  <th>{{ t('docs.hdrThDesc') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td><code class="docs-code">X-TraceHub-Session</code></td>
                  <td>{{ t('docs.hdrRow1d') }}</td>
                </tr>
                <tr>
                  <td><code class="docs-code">X-Sticky-Session</code></td>
                  <td>{{ t('docs.hdrRow2d') }}</td>
                </tr>
                <tr>
                  <td><code class="docs-code">X-TraceHub-Model</code></td>
                  <td>{{ t('docs.hdrRow3d') }}</td>
                </tr>
              </tbody>
            </table>
            <p class="mt-4 text-sm text-slate-500">{{ t('docs.hdrFoot') }}</p>
          </section>

          <section id="doc-routing" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>{{ t('docs.routingH2') }}</h2>
            <p class="mt-3 text-sm text-slate-400">{{ t('docs.routingIntro') }}</p>

            <h3 class="mt-8">{{ t('docs.routingUserH3') }}</h3>
            <p class="mt-3 text-sm text-slate-400">{{ t('docs.routingP1') }}</p>
            <ul class="mt-4 list-disc space-y-2 pl-5 text-sm text-slate-400">
              <li>{{ t('docs.routingLi1') }}</li>
              <li>{{ t('docs.routingLi2') }}</li>
              <li>{{ t('docs.routingLi3') }}</li>
              <li>{{ t('docs.routingLi3b') }}</li>
              <li>{{ t('docs.routingLi4') }}</li>
              <li>{{ t('docs.routingLi5') }}</li>
            </ul>
            <p class="mt-4 text-sm text-slate-500">{{ t('docs.routingUserFoot') }}</p>

            <h3 class="mt-8">{{ t('docs.routingFlowH3') }}</h3>
            <ol class="docs-ol mt-3 text-sm text-slate-400">
              <li>{{ t('docs.routingFlow1') }}</li>
              <li>{{ t('docs.routingFlow2') }}</li>
              <li>{{ t('docs.routingFlow3') }}</li>
              <li>{{ t('docs.routingFlow3b') }}</li>
              <li>{{ t('docs.routingFlow4') }}</li>
              <li>{{ t('docs.routingFlow5') }}</li>
            </ol>

            <h3 class="mt-8">{{ t('docs.routingStratH3') }}</h3>
            <table class="docs-table mt-4 text-sm">
              <thead>
                <tr>
                  <th>{{ t('docs.routingStratTh1') }}</th>
                  <th>{{ t('docs.routingStratTh2') }}</th>
                  <th>{{ t('docs.routingStratTh3') }}</th>
                </tr>
              </thead>
              <tbody class="text-slate-400">
                <tr>
                  <td class="font-mono text-slate-300">{{ t('docs.routingStratRow1n') }}</td>
                  <td>{{ t('docs.routingStratRow1d') }}</td>
                  <td>{{ t('docs.routingStratRow1p') }}</td>
                </tr>
                <tr>
                  <td class="font-mono text-slate-300">{{ t('docs.routingStratRow2n') }}</td>
                  <td>{{ t('docs.routingStratRow2d') }}</td>
                  <td>{{ t('docs.routingStratRow2p') }}</td>
                </tr>
                <tr>
                  <td class="font-mono text-slate-300">{{ t('docs.routingStratRow3n') }}</td>
                  <td>{{ t('docs.routingStratRow3d') }}</td>
                  <td>{{ t('docs.routingStratRow3p') }}</td>
                </tr>
              </tbody>
            </table>

            <h3 class="mt-8">{{ t('docs.routingFailoverH3') }}</h3>
            <p class="mt-3 text-sm text-slate-400">{{ t('docs.routingFailoverP') }}</p>
            <ul class="mt-4 list-disc space-y-2 pl-5 text-sm text-slate-400">
              <li>{{ t('docs.routingFailoverLi1') }}</li>
              <li>{{ t('docs.routingFailoverLi2') }}</li>
              <li>{{ t('docs.routingFailoverLi3') }}</li>
            </ul>

            <h3 class="mt-8">{{ t('docs.routingHealthH3') }}</h3>
            <p class="mt-3 text-sm text-slate-400">{{ t('docs.routingHealthP') }}</p>
            <ul class="mt-4 list-disc space-y-2 pl-5 text-sm text-slate-400">
              <li>{{ t('docs.routingHealthLi1') }}</li>
              <li>{{ t('docs.routingHealthLi2') }}</li>
            </ul>

            <h3 class="mt-8">{{ t('docs.routingStickyH3') }}</h3>
            <p class="mt-3 text-sm text-slate-400">{{ t('docs.routingStickyP') }}</p>

            <h3 class="mt-8">{{ t('docs.routingAdminH3') }}</h3>
            <p class="mt-3 text-sm text-slate-400">{{ t('docs.routingAdminIntro') }}</p>
            <h4 class="mt-6 text-sm font-medium text-slate-300">{{ t('docs.routingAdminGlobalH4') }}</h4>
            <ul class="mt-3 list-disc space-y-2 pl-5 text-sm text-slate-400">
              <li>{{ t('docs.routingAdminGlobal1') }}</li>
              <li>{{ t('docs.routingAdminGlobal2') }}</li>
              <li>{{ t('docs.routingAdminGlobal3') }}</li>
              <li>{{ t('docs.routingAdminGlobal4') }}</li>
              <li>{{ t('docs.routingAdminGlobal5') }}</li>
              <li>{{ t('docs.routingAdminGlobal6') }}</li>
            </ul>
            <h4 class="mt-6 text-sm font-medium text-slate-300">{{ t('docs.routingAdminGroupH4') }}</h4>
            <ul class="mt-3 list-disc space-y-2 pl-5 text-sm text-slate-400">
              <li>{{ t('docs.routingAdminGroup1') }}</li>
              <li>{{ t('docs.routingAdminGroup2') }}</li>
              <li>{{ t('docs.routingAdminGroup3') }}</li>
              <li>{{ t('docs.routingAdminGroup4') }}</li>
            </ul>
            <h4 class="mt-6 text-sm font-medium text-slate-300">{{ t('docs.routingAdminChannelH4') }}</h4>
            <ul class="mt-3 list-disc space-y-2 pl-5 text-sm text-slate-400">
              <li>{{ t('docs.routingAdminChannel1') }}</li>
              <li>{{ t('docs.routingAdminChannel2') }}</li>
              <li>{{ t('docs.routingAdminChannel3') }}</li>
            </ul>

            <h3 class="mt-8">{{ t('docs.routingLimitH3') }}</h3>
            <ul class="mt-4 list-disc space-y-2 pl-5 text-sm text-slate-400">
              <li>{{ t('docs.routingLimitLi1') }}</li>
              <li>{{ t('docs.routingLimitLi2') }}</li>
              <li>{{ t('docs.routingLimitLi3') }}</li>
            </ul>
            <p class="mt-4 text-sm text-slate-500">{{ t('docs.routingFoot') }}</p>
          </section>

          <section id="doc-user-api" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>{{ t('docs.userApiH2') }}</h2>
            <p class="text-sm text-slate-400">{{ t('docs.userApiSub') }}</p>
            <table class="docs-table mt-4 text-left text-xs">
              <thead>
                <tr>
                  <th>{{ t('docs.tblMethod') }}</th>
                  <th>{{ t('docs.tblPath') }}</th>
                  <th>{{ t('docs.tblNote') }}</th>
                </tr>
              </thead>
              <tbody class="text-slate-400">
                <tr><td class="text-emerald-400/90">POST</td><td class="font-mono text-slate-300">/api/user/register</td><td>{{ t('docs.uaReg') }}</td></tr>
                <tr><td class="text-emerald-400/90">POST</td><td class="font-mono text-slate-300">/api/user/login</td><td>{{ t('docs.uaLogin') }}</td></tr>
                <tr><td class="text-emerald-400/90">POST</td><td class="font-mono text-slate-300">/api/user/refresh</td><td>{{ t('docs.uaRefresh') }}</td></tr>
                <tr><td class="text-sky-400/90">GET</td><td class="font-mono text-slate-300">/api/user/me</td><td>{{ t('docs.uaMe') }}</td></tr>
                <tr><td class="text-sky-400/90">GET</td><td class="font-mono text-slate-300">/api/user/balance</td><td>{{ t('docs.uaBal') }}</td></tr>
                <tr><td class="text-sky-400/90">GET</td><td class="font-mono text-slate-300">/api/user/usage-logs</td><td>{{ t('docs.uaLogs') }}</td></tr>
                <tr><td class="text-sky-400/90">GET</td><td class="font-mono text-slate-300">/api/user/balance-ledger</td><td>{{ t('docs.uaLedger') }}</td></tr>
                <tr><td class="text-sky-400/90">GET</td><td class="font-mono text-slate-300">/api/user/group</td><td>{{ t('docs.uaGrp') }}</td></tr>
                <tr><td class="text-sky-400/90">GET</td><td class="font-mono text-slate-300">/api/user/available-models</td><td>{{ t('docs.uaAvail') }}</td></tr>
                <tr><td class="text-sky-400/90">GET</td><td class="font-mono text-slate-300">/api/user/usage</td><td>{{ t('docs.uaUsage') }}</td></tr>
                <tr><td class="text-sky-400/90">GET</td><td class="font-mono text-slate-300">/api/user/usage/export</td><td>{{ t('docs.uaExport') }}</td></tr>
                <tr><td class="text-sky-400/90">GET</td><td class="font-mono text-slate-300">/api/user/apikeys</td><td>{{ t('docs.uaKeysList') }}</td></tr>
                <tr><td class="text-emerald-400/90">POST</td><td class="font-mono text-slate-300">/api/user/apikeys</td><td>{{ t('docs.uaKeysCreate') }}</td></tr>
                <tr><td class="text-amber-400/90">PUT</td><td class="font-mono text-slate-300">/api/user/apikeys/:id</td><td>{{ t('docs.uaKeysPut') }}</td></tr>
                <tr><td class="text-rose-400/90">DELETE</td><td class="font-mono text-slate-300">/api/user/apikeys/:id</td><td>{{ t('docs.uaKeysDel') }}</td></tr>
                <tr><td class="text-emerald-400/90">POST</td><td class="font-mono text-slate-300">/api/user/redeem</td><td>{{ t('docs.uaRedeem') }}</td></tr>
                <tr><td class="text-sky-400/90">GET</td><td class="font-mono text-slate-300">/api/user/redeem-ledger</td><td>{{ t('docs.uaRedeemLedger') }}</td></tr>
                <tr><td class="text-amber-400/90">PUT</td><td class="font-mono text-slate-300">/api/user/password</td><td>{{ t('docs.uaPwd') }}</td></tr>
                <tr><td class="text-amber-400/90">PUT</td><td class="font-mono text-slate-300">/api/user/bind</td><td>{{ t('docs.uaBind') }}</td></tr>
                <tr><td class="text-amber-400/90">PUT</td><td class="font-mono text-slate-300">/api/user/ip_whitelist</td><td>{{ t('docs.uaIp') }}</td></tr>
              </tbody>
            </table>
          </section>

          <section id="doc-admin-api" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>{{ t('docs.adminApiH2') }}</h2>
            <p>{{ t('docs.adminApiP1') }}</p>
            <p class="mt-3 text-sm text-slate-500">{{ t('docs.adminApiP2') }}</p>
          </section>

          <section id="doc-ops" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>{{ t('docs.opsH2') }}</h2>
            <ul class="docs-endpoint-list font-mono text-xs">
              <li>{{ t('docs.opHealth') }}</li>
              <li>{{ t('docs.opReady') }}</li>
              <li>{{ t('docs.opMetrics') }}</li>
            </ul>
            <p class="mt-4 text-sm text-slate-500">{{ t('docs.opsFoot') }}</p>
          </section>

          <section id="doc-errors" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>{{ t('docs.errH2') }}</h2>
            <p>{{ t('docs.errIntro') }}</p>
            <ul class="docs-ul mt-3">
              <li>{{ t('docs.err401') }}</li>
              <li>{{ t('docs.err402') }}</li>
              <li>{{ t('docs.err403') }}</li>
              <li>{{ t('docs.err400') }}</li>
            </ul>
            <p class="mt-4 text-sm text-slate-500">{{ t('docs.errFoot') }}</p>
          </section>

          <section id="doc-faq" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>{{ t('docs.faqH2') }}</h2>
            <dl class="docs-dl mt-4">
              <dt>{{ t('docs.fq1q') }}</dt>
              <dd>{{ t('docs.fq1a') }}</dd>
              <dt>{{ t('docs.fq2q') }}</dt>
              <dd>{{ t('docs.fq2a', { base: siteMeta.effectiveOpenaiApiBase }) }}</dd>
              <dt>{{ t('docs.fq3q') }}</dt>
              <dd>{{ t('docs.fq3a') }}</dd>
              <dt>{{ t('docs.fq4q') }}</dt>
              <dd>{{ t('docs.fq4a') }}</dd>
              <dt>{{ t('docs.fq5q') }}</dt>
              <dd>{{ t('docs.fq5a') }}</dd>
              <dt>{{ t('docs.fq6q') }}</dt>
              <dd>{{ t('docs.fq6a') }}</dd>
              <dt>{{ t('docs.fq7q') }}</dt>
              <dd>{{ t('docs.fq7a') }}</dd>
              <dt>{{ t('docs.fq8q') }}</dt>
              <dd>{{ t('docs.fq8a') }}</dd>
              <dt>{{ t('docs.fq9q') }}</dt>
              <dd>{{ t('docs.fq9a') }}</dd>
            </dl>
            <div class="mt-6 border-t border-white/[0.06] pt-6">
              <ContactCardDisclosure align="start" summary-key="home.contactHintDocsFaq" />
            </div>
          </section>
        </article>
      </main>
    </div>

    <footer class="docs-chrome-border-t py-10 text-center">
      <RouterLink to="/" class="text-sm text-slate-500 transition hover:text-slate-400">{{ t('docs.footerHome') }}</RouterLink>
      <p class="mt-3 text-[11px] text-slate-600">{{ t('docs.footerCompany') }}</p>
    </footer>
  </div>
</template>

<style scoped>
.docs-code {
  border-radius: 0.25rem;
  border: 1px solid var(--th-docs-code-border);
  background: var(--th-docs-code-bg);
  padding: 0.1rem 0.4rem;
  font-family: var(--th-font-mono);
  font-size: 0.75rem;
  color: var(--th-docs-code-fg);
}
.docs-pre {
  overflow-x: auto;
  border-radius: 0.5rem;
  border: 1px solid var(--th-docs-pre-border);
  background: var(--th-docs-pre-bg);
  padding: 0.875rem 1rem;
  font-family: var(--th-font-mono);
  font-size: 0.75rem;
  line-height: 1.55;
  color: var(--th-docs-pre-fg);
}
.docs-prose .docs-section h2 {
  margin: 0;
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--th-docs-h2);
  letter-spacing: -0.02em;
}
.docs-prose .docs-section h3 {
  margin: 0;
  margin-top: 1rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--th-docs-h3);
}
.docs-prose .docs-section p {
  margin-top: 0.75rem;
  font-size: 0.875rem;
  line-height: 1.65;
  color: var(--th-docs-body);
}
.docs-prose .docs-section p:first-of-type {
  margin-top: 1rem;
}
.docs-ol {
  list-style: decimal;
  padding-left: 1.25rem;
  font-size: 0.875rem;
  line-height: 1.65;
  color: var(--th-docs-body);
}
.docs-ol li {
  margin-top: 0.5rem;
}
.docs-ul {
  list-style: disc;
  padding-left: 1.25rem;
  font-size: 0.875rem;
  line-height: 1.65;
  color: var(--th-docs-body);
}
.docs-ul li {
  margin-top: 0.35rem;
}
.docs-dl dt {
  margin-top: 1rem;
  font-size: 0.8125rem;
  font-weight: 600;
  color: var(--th-docs-dt);
}
.docs-dl dt:first-child {
  margin-top: 0;
}
.docs-dl dd {
  margin-top: 0.35rem;
  font-size: 0.8125rem;
  line-height: 1.6;
  color: var(--th-docs-body);
}
.docs-callout {
  font-size: 0.8125rem;
  line-height: 1.6;
  color: var(--th-docs-body);
}
.docs-endpoint-list {
  margin-top: 0.5rem;
  list-style: none;
  padding: 0;
}
.docs-endpoint-list.font-mono {
  color: var(--th-docs-endpoint-list-fg);
}
.docs-endpoint-list li {
  margin-top: 0.35rem;
  padding: 0.35rem 0.5rem;
  border-radius: 0.375rem;
  background: var(--th-docs-endpoint-li-bg);
}
.docs-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8125rem;
}
.docs-table th,
.docs-table td {
  border: 1px solid var(--th-docs-table-border);
  padding: 0.5rem 0.65rem;
  vertical-align: top;
}
.docs-table td {
  color: var(--th-docs-body);
}
.docs-table th {
  background: var(--th-docs-table-th-bg);
  color: var(--th-docs-table-th-fg);
  font-weight: 600;
  text-align: left;
}
.docs-toc-link {
  border-radius: 0.375rem;
  color: var(--th-docs-toc-fg);
  transition:
    color 0.15s,
    background 0.15s,
    border-color 0.15s;
}
.docs-toc-link:hover {
  color: var(--th-docs-toc-hover-fg);
  background: var(--th-docs-toc-hover-bg);
}
.docs-toc-link--active {
  color: var(--th-docs-toc-active-fg);
  background: var(--th-docs-toc-active-bg);
}
.docs-aside .docs-toc-link {
  color: var(--th-docs-toc-fg);
}
.docs-aside .docs-toc-link:hover {
  color: var(--th-docs-toc-hover-fg);
}
.docs-aside .docs-toc-link--active {
  border-left-color: var(--th-docs-toc-active-border);
}
.docs-mobile-nav {
  border: 1px solid var(--th-docs-mobile-nav-border);
  background: var(--th-docs-mobile-nav-bg);
}
.docs-mobile-toc-btn {
  border: 1px solid var(--th-docs-mobile-toc-border);
  background: var(--th-docs-mobile-toc-bg);
  color: var(--th-text);
  transition:
    background 0.15s,
    border-color 0.15s;
}
.docs-mobile-toc-btn:hover {
  background: var(--th-docs-mobile-toc-hover-bg);
}
.docs-admin-note {
  border: 1px solid var(--th-docs-admin-border);
  background: var(--th-docs-admin-bg);
  color: var(--th-docs-admin-fg);
}
</style>
