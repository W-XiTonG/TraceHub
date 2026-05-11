<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import UserAccountMenu from '../components/UserAccountMenu.vue'

const { t } = useI18n()
const auth = useAuthStore()
const version = ref('1.0.0')

let revealObserver: IntersectionObserver | null = null

onMounted(() => {
  void nextTick(() => {
    const nodes = document.querySelectorAll<HTMLElement>('.th-reveal')
    if (!nodes.length) return

    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      nodes.forEach((el) => el.classList.add('th-reveal--in'))
      return
    }

    revealObserver = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          if (e.isIntersecting && e.target instanceof HTMLElement) {
            e.target.classList.add('th-reveal--in')
            revealObserver?.unobserve(e.target)
          }
        }
      },
      { rootMargin: '0px 0px -8% 0px', threshold: 0.07 },
    )
    nodes.forEach((el) => revealObserver!.observe(el))
  })
})

onBeforeUnmount(() => {
  revealObserver?.disconnect()
  revealObserver = null
})

const models = [
  'gpt-4o',
  'gpt-4o-mini',
  'claude-3-5-sonnet-20241022',
  'gemini-2.0-flash',
  'deepseek-chat',
  'qwen-plus',
]

const faqs = computed(() => [
  { q: t('home.faq0q'), a: t('home.faq0a') },
  { q: t('home.faq1q'), a: t('home.faq1a') },
  { q: t('home.faq2q'), a: t('home.faq2a') },
  { q: t('home.faq3q'), a: t('home.faq3a') },
  { q: t('home.faq4q'), a: t('home.faq4a') },
])

type FeatureAccent = 'cyan' | 'violet' | 'amber' | 'emerald' | 'sky' | 'fuchsia'

const features = computed<
  Array<{
    iconId: string
    tag: string
    title: string
    desc: string
    to: string
    cta: string
    accent: FeatureAccent
  }>
>(() => [
  {
    iconId: 'plug',
    tag: t('home.f0tag'),
    title: t('home.f0title'),
    desc: t('home.f0desc'),
    to: '/console/models',
    cta: t('home.f0cta'),
    accent: 'cyan',
  },
  {
    iconId: 'key',
    tag: t('home.f1tag'),
    title: t('home.f1title'),
    desc: t('home.f1desc'),
    to: '/console/keys',
    cta: t('home.f1cta'),
    accent: 'violet',
  },
  {
    iconId: 'chart',
    tag: t('home.f2tag'),
    title: t('home.f2title'),
    desc: t('home.f2desc'),
    to: '/console/logs',
    cta: t('home.f2cta'),
    accent: 'emerald',
  },
  {
    iconId: 'users',
    tag: t('home.f3tag'),
    title: t('home.f3title'),
    desc: t('home.f3desc'),
    to: '/console/group',
    cta: t('home.f3cta'),
    accent: 'sky',
  },
  {
    iconId: 'wallet',
    tag: t('home.f4tag'),
    title: t('home.f4title'),
    desc: t('home.f4desc'),
    to: '/console',
    cta: t('home.f4cta'),
    accent: 'amber',
  },
  {
    iconId: 'shield',
    tag: t('home.f5tag'),
    title: t('home.f5title'),
    desc: t('home.f5desc'),
    to: '/admin',
    cta: t('home.f5cta'),
    accent: 'fuchsia',
  },
])

const accentRing: Record<FeatureAccent, string> = {
  cyan: 'group-hover:border-cyan-500/25 group-hover:shadow-cyan-500/10',
  violet: 'group-hover:border-violet-500/25 group-hover:shadow-violet-500/10',
  amber: 'group-hover:border-amber-500/25 group-hover:shadow-amber-500/10',
  emerald: 'group-hover:border-emerald-500/25 group-hover:shadow-emerald-500/10',
  sky: 'group-hover:border-sky-500/25 group-hover:shadow-sky-500/10',
  fuchsia: 'group-hover:border-fuchsia-500/25 group-hover:shadow-fuchsia-500/10',
}

const accentIconBg: Record<FeatureAccent, string> = {
  cyan: 'bg-cyan-500/15 text-cyan-200',
  violet: 'bg-violet-500/15 text-violet-200',
  amber: 'bg-amber-500/15 text-amber-200',
  emerald: 'bg-emerald-500/15 text-emerald-200',
  sky: 'bg-sky-500/15 text-sky-200',
  fuchsia: 'bg-fuchsia-500/15 text-fuchsia-200',
}

const trustPoints = computed(() => [
  { label: t('home.t0label'), hint: t('home.t0hint') },
  { label: t('home.t1label'), hint: t('home.t1hint') },
  { label: t('home.t2label'), hint: t('home.t2hint') },
  { label: t('home.t3label'), hint: t('home.t3hint') },
])

function featureLink(to: string) {
  if (auth.isLoggedIn) return { path: to }
  return { path: '/login', query: { redirect: to } }
}
</script>

<template>
  <div class="th-page-shell th-home text-slate-100">
    <div class="th-home-ambient" aria-hidden="true">
      <div class="th-home-blob th-home-blob--1" />
      <div class="th-home-blob th-home-blob--2" />
      <div class="th-home-blob th-home-blob--3" />
      <svg
        class="th-home-mesh"
        xmlns="http://www.w3.org/2000/svg"
        preserveAspectRatio="none"
        viewBox="0 0 1200 800"
      >
        <defs>
          <pattern id="th-home-line-grid" width="72" height="72" patternUnits="userSpaceOnUse">
            <path
              d="M 72 0 L 0 0 0 72"
              fill="none"
              stroke="rgba(148,163,184,0.12)"
              stroke-width="0.55"
            />
          </pattern>
          <linearGradient id="th-home-mesh-grad" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stop-color="var(--th-mesh-stop-1)" />
            <stop offset="50%" stop-color="var(--th-mesh-stop-2)" />
            <stop offset="100%" stop-color="var(--th-mesh-stop-3)" />
          </linearGradient>
        </defs>
        <rect width="100%" height="100%" fill="url(#th-home-line-grid)" />
        <rect width="100%" height="100%" fill="url(#th-home-mesh-grad)" opacity="0.75" style="mix-blend-mode: soft-light" />
      </svg>
      <div class="th-home-radial-vignette" />
    </div>

    <header class="th-site-header th-home-header-enter">
      <div class="mx-auto flex max-w-6xl items-center justify-between px-5 py-4">
        <div class="flex items-center gap-3">
          <div class="th-logo-mark th-logo-mark--sm shrink-0">TH</div>
          <div>
            <span class="th-sidebar-brand block text-lg font-semibold tracking-tight text-white">TraceHub</span>
            <span class="text-[10px] font-medium uppercase tracking-[0.18em] text-slate-500">{{ t('home.brandSub') }}</span>
          </div>
          <span class="ml-1 rounded border border-white/[0.08] bg-white/[0.04] px-2 py-0.5 font-mono text-[10px] text-slate-500">v{{ version }}</span>
        </div>
        <nav class="flex flex-wrap items-center justify-end gap-0.5 sm:gap-1">
          <a href="#features" class="th-site-nav-link">{{ t('nav.features') }}</a>
          <a href="#integration" class="th-site-nav-link hidden sm:inline">{{ t('nav.integration') }}</a>
          <a href="#models" class="th-site-nav-link">{{ t('nav.models') }}</a>
          <a href="#faq" class="th-site-nav-link">{{ t('nav.faq') }}</a>
          <RouterLink to="/docs" class="th-site-nav-link">{{ t('nav.docs') }}</RouterLink>
          <template v-if="!auth.isLoggedIn">
            <RouterLink to="/login" class="th-site-nav-link text-slate-300">{{ t('nav.login') }}</RouterLink>
            <RouterLink to="/register" class="th-btn-primary !px-4 !py-2 text-sm">{{ t('nav.register') }}</RouterLink>
          </template>
          <UserAccountMenu v-else accent="cyan" />
        </nav>
      </div>
    </header>

    <main class="relative z-[1]">
      <section class="relative mx-auto max-w-6xl overflow-hidden px-5 pb-24 pt-20 text-center md:pt-28">
        <div class="th-hero-backdrop" aria-hidden="true" />
        <div class="relative z-10 th-stagger-children">
        <p class="th-kicker mb-2"><span class="th-kicker-accent">TraceHub</span> · {{ t('home.heroKickerTail') }}</p>
        <h1 class="relative mx-auto max-w-4xl text-4xl font-semibold leading-[1.12] tracking-tight md:text-[3.25rem] md:leading-[1.1]">
          <span class="th-headline-gradient">{{ t('home.heroLine1') }}</span>
          <span class="text-slate-200">{{ t('home.heroLine2') }}</span>
        </h1>
        <p class="relative mx-auto mt-6 max-w-2xl text-base leading-relaxed text-slate-500 md:text-[1.0625rem] md:leading-relaxed">
          {{ t('home.heroDesc') }}
        </p>
        <div class="relative mt-10 flex flex-wrap justify-center gap-3 sm:gap-4">
          <RouterLink
            :to="auth.isLoggedIn ? '/console' : '/register'"
            class="th-btn-primary px-8 py-3.5 text-base sm:px-10"
          >
            {{ auth.isLoggedIn ? t('home.ctaConsole') : t('home.ctaStart') }}
          </RouterLink>
          <RouterLink to="/docs#doc-console" class="th-btn-secondary px-8 py-3.5 text-base sm:px-10">{{
            t('home.ctaDocs')
          }}</RouterLink>
          <a href="#integration" class="th-btn-secondary px-8 py-3.5 text-base sm:px-10">{{ t('home.ctaIntegration') }}</a>
        </div>
        <ul
          class="th-stagger-trust relative mx-auto mt-14 flex max-w-4xl flex-wrap justify-center gap-2 md:gap-3"
          :aria-label="t('home.trustAria')"
        >
          <li
            v-for="t in trustPoints"
            :key="t.label"
            class="th-glass--subtle flex min-w-[10.5rem] flex-1 flex-col items-center justify-center rounded-xl px-4 py-3 text-center sm:min-w-0 sm:flex-none sm:px-5"
          >
            <span class="text-[13px] font-medium text-slate-300">{{ t.label }}</span>
            <span class="mt-0.5 text-[11px] text-slate-600">{{ t.hint }}</span>
          </li>
        </ul>
        </div>
      </section>

      <svg class="pointer-events-none absolute h-0 w-0 overflow-hidden" aria-hidden="true" focusable="false">
        <defs>
          <symbol id="th-icon-plug" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75v-6z" />
          </symbol>
          <symbol id="th-icon-key" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 5.25a3 3 0 013 3m3 0a6 6 0 01-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H3.75v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1121.75 8.25z" />
          </symbol>
          <symbol id="th-icon-chart" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z" />
          </symbol>
          <symbol id="th-icon-users" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M15 19.128a9.38 9.38 0 002.625.372 9.337 9.337 0 004.121-.952 4.125 4.125 0 00-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 018.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0111.964-3.07M12 6.375a3.375 3.375 0 11-6.75 0 3.375 3.375 0 016.75 0zm8.25 2.25a2.625 2.625 0 11-5.25 0 2.625 2.625 0 015.25 0z" />
          </symbol>
          <symbol id="th-icon-wallet" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M21 12a2.25 2.25 0 00-2.25-2.25H15a3 3 0 11-6 0H5.25A2.25 2.25 0 003 12m18 0v6a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 18v-6m18 0V9M3 12V9m18 3a2.25 2.25 0 01-2.25-2.25H15a3 3 0 10-6 0H5.25A2.25 2.25 0 003 12" />
          </symbol>
          <symbol id="th-icon-shield" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z" />
          </symbol>
        </defs>
      </svg>

      <section id="features" class="th-reveal relative py-20">
        <div class="relative mx-auto max-w-6xl px-5">
          <h2 class="text-center text-2xl font-semibold tracking-tight text-slate-100 md:text-[1.75rem]">
            {{ t('home.featuresTitle') }}
          </h2>
          <p class="mx-auto mt-3 max-w-2xl text-center text-sm leading-relaxed text-slate-500">
            {{ t('home.featuresDesc') }}
          </p>
          <div class="th-reveal-stagger mt-12 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <RouterLink
              v-for="f in features"
              :key="f.title"
              :to="featureLink(f.to)"
              class="group th-glass th-home-feature-card flex flex-col rounded-[var(--th-radius-xl)] p-6 no-underline transition duration-300 hover:-translate-y-1 hover:bg-white/[0.04] hover:shadow-lg"
              :class="accentRing[f.accent]"
            >
              <div class="flex items-start justify-between gap-3">
                <span
                  class="th-home-feature-icon inline-flex h-11 w-11 shrink-0 items-center justify-center rounded-xl [&>svg]:h-5 [&>svg]:w-5"
                  :class="[accentIconBg[f.accent], `th-home-feature-icon--${f.accent}`]"
                >
                  <svg><use :href="`#th-icon-${f.iconId}`" /></svg>
                </span>
                <span class="rounded-md border border-white/[0.08] bg-white/[0.04] px-2 py-0.5 font-mono text-[10px] font-medium uppercase tracking-wider text-slate-500">{{ f.tag }}</span>
              </div>
              <h3 class="mt-4 text-lg font-semibold tracking-tight text-slate-100">{{ f.title }}</h3>
              <p class="mt-2 flex-1 text-sm leading-relaxed text-slate-500">{{ f.desc }}</p>
              <span class="mt-5 inline-flex items-center gap-1 text-sm font-medium text-cyan-400/95 transition group-hover:text-cyan-300">
                {{ f.cta }}
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-4 w-4 translate-x-0 transition group-hover:translate-x-0.5" aria-hidden="true">
                  <path fill-rule="evenodd" d="M3 10a.75.75 0 01.75-.75h10.638L10.23 5.29a.75.75 0 111.04-1.08l5.5 5.25a.75.75 0 010 1.08l-5.5 5.25a.75.75 0 11-1.04-1.08l4.158-3.96H3.75A.75.75 0 013 10z" clip-rule="evenodd" />
                </svg>
              </span>
            </RouterLink>
          </div>
        </div>
      </section>

      <section
        id="integration"
        class="th-reveal relative border-y border-white/[0.08] bg-gradient-to-b from-white/[0.03] to-transparent py-20"
      >
        <div class="mx-auto max-w-6xl px-5">
          <div class="th-reveal-stagger th-reveal-stagger--integration grid gap-10 lg:grid-cols-2 lg:items-center">
            <div>
              <p class="th-kicker mb-2"><span class="th-kicker-accent">{{ t('home.integrationKicker') }}</span></p>
              <h2 class="text-2xl font-semibold tracking-tight text-slate-100 md:text-[1.75rem]">{{ t('home.integrationTitle') }}</h2>
              <p class="mt-3 text-sm leading-relaxed text-slate-500">
                {{ t('home.integrationDesc') }}
              </p>
              <ul class="mt-6 space-y-3 text-sm text-slate-400">
                <li class="flex gap-2">
                  <span class="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-cyan-500/80" aria-hidden="true" />
                  <span>{{ t('home.integrationLi1', { getModels: 'GET /v1/models' }) }}</span>
                </li>
                <li class="flex gap-2">
                  <span class="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-cyan-500/80" aria-hidden="true" />
                  <span>{{ t('home.integrationLi2', { postChat: 'POST /v1/chat/completions' }) }}</span>
                </li>
                <li class="flex gap-2">
                  <span class="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-cyan-500/80" aria-hidden="true" />
                  <span>{{ t('home.integrationLi3', { metrics: '/metrics' }) }}</span>
                </li>
              </ul>
            </div>
            <div class="th-glass th-home-code-window overflow-hidden rounded-[var(--th-radius-xl)] p-1">
              <div class="th-home-code-window-titlebar flex items-center gap-2 border-b border-white/[0.06] bg-slate-950/80 px-4 py-2.5">
                <span class="h-2.5 w-2.5 rounded-full bg-red-500/70" aria-hidden="true" />
                <span class="h-2.5 w-2.5 rounded-full bg-amber-500/70" aria-hidden="true" />
                <span class="h-2.5 w-2.5 rounded-full bg-emerald-500/70" aria-hidden="true" />
                <span class="ml-2 font-mono text-[11px] text-slate-600">{{ t('home.codeWinTitle') }}</span>
              </div>
              <pre class="th-home-code-window-pre max-h-[min(22rem,52vh)] overflow-auto bg-[#050a14] p-4 font-mono text-[11px] leading-relaxed text-slate-400 md:text-xs"><code><span class="th-home-code-comment text-slate-600">{{ t('home.codeComment') }}</span>
curl -sS <span class="th-home-code-str text-emerald-400/90">"$BASE/v1/chat/completions"</span> \
  -H <span class="th-home-code-hdr text-amber-200/85">"Authorization: Bearer $KEY"</span> \
  -H <span class="th-home-code-hdr text-amber-200/85">"Content-Type: application/json"</span> \
  -d <span class="th-home-code-json text-sky-200/90">'{
    "model": "gpt-4o-mini",
    "messages": [{ "role": "user", "content": "Hello from TraceHub" }]
  }'</span></code></pre>
            </div>
          </div>
        </div>
      </section>

      <section
        id="models"
        class="th-reveal relative border-y border-white/[0.08] bg-gradient-to-b from-white/[0.02] to-transparent py-20"
      >
        <div class="relative mx-auto max-w-6xl px-5">
          <h2 class="text-center text-2xl font-semibold tracking-tight text-slate-100 md:text-[1.75rem]">{{ t('home.modelsTitle') }}</h2>
          <p class="mx-auto mt-3 max-w-2xl text-center text-sm leading-relaxed text-slate-500">
            {{ t('home.modelsDesc') }}
          </p>
          <p class="mx-auto mt-3 max-w-2xl text-center text-xs leading-relaxed text-slate-600">
            {{ t('home.modelsNote') }}
          </p>
          <div class="th-reveal-stagger th-reveal-stagger--tight mt-6 flex flex-wrap items-center justify-center gap-3 text-sm">
            <RouterLink
              to="/docs#doc-gateway"
              class="rounded-lg border border-white/[0.08] bg-white/[0.03] px-3 py-1.5 text-slate-300 transition hover:border-cyan-500/25 hover:bg-cyan-950/20 hover:text-cyan-200/95"
            >
              {{ t('home.modelsLinkDocs') }}
            </RouterLink>
            <RouterLink
              to="/console/models"
              class="rounded-lg border border-white/[0.08] bg-white/[0.03] px-3 py-1.5 text-slate-300 transition hover:border-cyan-500/25 hover:bg-cyan-950/20 hover:text-cyan-200/95"
            >
              {{ t('home.modelsLinkConsole') }}
            </RouterLink>
          </div>
          <div class="th-reveal-stagger th-reveal-stagger--tight mt-10 flex flex-col items-center gap-3">
            <span
              class="rounded-full border border-amber-500/25 bg-amber-950/25 px-3 py-1 text-[11px] font-medium uppercase tracking-wider text-amber-200/90"
            >
              {{ t('home.modelsBadge') }}
            </span>
            <div class="th-home-chip-row flex max-w-3xl flex-wrap justify-center gap-2">
              <span v-for="m in models" :key="m" class="th-chip">{{ m }}</span>
              <span
                class="th-chip border border-dashed border-slate-600/70 bg-transparent font-sans text-slate-500 italic ring-0"
              >{{ t('home.modelsMore') }}</span>
            </div>
          </div>
        </div>
      </section>

      <section id="pricing" class="th-reveal mx-auto max-w-6xl px-5 py-16 md:py-20">
        <div class="th-glass--subtle mx-auto max-w-2xl rounded-[var(--th-radius-xl)] px-6 py-8 text-center md:px-10 md:py-10">
          <h2 class="text-lg font-semibold tracking-tight text-slate-200 md:text-xl">{{ t('home.usageNoteTitle') }}</h2>
          <p class="mx-auto mt-3 max-w-lg text-sm leading-relaxed text-slate-500">
            {{ t('home.usageNoteBody') }}
          </p>
          <RouterLink
            to="/docs#doc-console"
            class="mt-6 inline-flex items-center gap-1.5 rounded-lg border border-cyan-500/25 bg-cyan-950/25 px-4 py-2.5 text-sm font-medium text-cyan-200/95 transition hover:border-cyan-400/35 hover:bg-cyan-900/30"
          >
            {{ t('home.usageNoteCta') }}
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-4 w-4" aria-hidden="true">
              <path
                fill-rule="evenodd"
                d="M3 10a.75.75 0 01.75-.75h10.638L10.23 5.29a.75.75 0 111.04-1.08l5.5 5.25a.75.75 0 010 1.08l-5.5 5.25a.75.75 0 11-1.04-1.08l4.158-3.96H3.75A.75.75 0 013 10z"
                clip-rule="evenodd"
              />
            </svg>
          </RouterLink>
        </div>
      </section>

      <section id="faq" class="th-reveal border-t border-white/[0.06] py-20">
        <div class="mx-auto max-w-3xl px-5">
          <h2 class="text-center text-2xl font-semibold tracking-tight text-slate-100 md:text-[1.75rem]">{{ t('home.faqTitle') }}</h2>
          <div class="th-reveal-stagger th-reveal-stagger--faq mt-10 space-y-3">
            <div v-for="(f, i) in faqs" :key="i" class="th-glass th-faq-item rounded-[var(--th-radius-lg)] p-6">
              <p class="text-[15px] font-medium leading-snug text-slate-200">{{ f.q }}</p>
              <p class="mt-3 text-sm leading-relaxed text-slate-500">{{ f.a }}</p>
            </div>
          </div>
        </div>
      </section>
    </main>

    <footer class="th-reveal relative z-[1] border-t border-white/[0.06] py-14">
      <div class="mx-auto flex max-w-6xl flex-col gap-10 px-5 md:flex-row md:justify-between md:gap-12">
        <div class="max-w-sm">
          <div class="flex items-center gap-2">
            <div class="th-logo-mark th-logo-mark--sm shrink-0">TH</div>
            <span class="th-sidebar-brand text-base font-semibold text-slate-200">TraceHub</span>
          </div>
          <p class="mt-3 text-sm leading-relaxed text-slate-500">
            {{ t('home.footerBlurb') }}
          </p>
        </div>
        <div class="grid grid-cols-2 gap-10 sm:grid-cols-3 md:gap-14">
          <div>
            <p class="text-[11px] font-semibold uppercase tracking-widest text-slate-600">{{ t('nav.product') }}</p>
            <ul class="mt-3 space-y-2 text-sm text-slate-400">
              <li><a href="#features" class="transition hover:text-cyan-400/90">{{ t('home.footerFeat') }}</a></li>
              <li><a href="#integration" class="transition hover:text-cyan-400/90">{{ t('home.footerInteg') }}</a></li>
              <li><a href="#models" class="transition hover:text-cyan-400/90">{{ t('home.footerModels') }}</a></li>
              <li>
                <RouterLink to="/docs#doc-console" class="transition hover:text-cyan-400/90">{{ t('home.footerDocsConsole') }}</RouterLink>
              </li>
            </ul>
          </div>
          <div>
            <p class="text-[11px] font-semibold uppercase tracking-widest text-slate-600">{{ t('nav.account') }}</p>
            <ul class="mt-3 space-y-2 text-sm text-slate-400">
              <li><RouterLink to="/register" class="transition hover:text-cyan-400/90">{{ t('nav.register') }}</RouterLink></li>
              <li><RouterLink to="/login" class="transition hover:text-cyan-400/90">{{ t('nav.login') }}</RouterLink></li>
              <li><RouterLink to="/console" class="transition hover:text-cyan-400/90">{{ t('userMenu.console') }}</RouterLink></li>
            </ul>
          </div>
          <div class="col-span-2 sm:col-span-1">
            <p class="text-[11px] font-semibold uppercase tracking-widest text-slate-600">{{ t('nav.docCol') }}</p>
            <ul class="mt-3 space-y-2 text-sm text-slate-400">
              <li>
                <span class="leading-relaxed">{{ t('home.footerDocLine1') }}</span>
                <code class="rounded bg-white/[0.06] px-1.5 py-0.5 font-mono text-[11px] text-slate-500">docs/</code>
                <span class="leading-relaxed">{{ t('home.footerDocLine2') }}</span>
              </li>
            </ul>
          </div>
        </div>
      </div>
      <p class="mx-auto mt-10 max-w-6xl border-t border-white/[0.05] px-5 pt-8 text-center text-xs leading-relaxed text-slate-600">
        {{ t('home.footerLegal', { metrics: '/metrics', health: '/health', ready: '/ready' }) }}
      </p>
    </footer>
  </div>
</template>
