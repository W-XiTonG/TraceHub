<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import UserAccountMenu from '../components/UserAccountMenu.vue'

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

const faqs = [
  {
    q: '计费是否透明？',
    a: '按 Token 精确计费，费率在控制台与官网公示，无隐藏扣费。',
  },
  {
    q: '支持流式输出吗？',
    a: 'OpenAI 兼容渠道支持 SSE 流式；Claude / Gemini 等渠道可输出 OpenAI 风格的 chat.completion.chunk 流（以部署版本为准）。',
  },
  {
    q: '数据安全如何保障？',
    a: '上游密钥 AES-GCM 加密存储，API Key 仅保存哈希；支持私有化部署与审计日志扩展。',
  },
  {
    q: '控制台能做什么？',
    a: '可用模型查询、创建/轮换 API Key、按周期查看调用日志、分组与倍率、充值与兑换码、用量导出（CSV/JSON），管理员另有渠道与模型市场等后台能力。',
  },
  {
    q: '和直接调用上游有什么区别？',
    a: '统一域名与鉴权、按账号聚合用量与账单、渠道路由与加权选择、可选 Redis 会话粘性；便于团队治理与后续配额、审计扩展。',
  },
]

type FeatureAccent = 'cyan' | 'violet' | 'amber' | 'emerald' | 'sky' | 'fuchsia'

const features: Array<{
  iconId: string
  tag: string
  title: string
  desc: string
  to: string
  cta: string
  accent: FeatureAccent
}> = [
  {
    iconId: 'plug',
    tag: 'Gateway',
    title: 'OpenAI 兼容接入',
    desc: '同一套 /v1/chat/completions 与 /v1/models，后端对接多上游；支持 SSE 流式与可选路径透传（按站点配置）。',
    to: '/console/models',
    cta: '查看可用模型',
    accent: 'cyan',
  },
  {
    iconId: 'key',
    tag: '安全',
    title: 'API Key 与鉴权',
    desc: '用户级密钥仅存储哈希；请求携带 Bearer Token 即可调用网关，支持在控制台随时轮换。',
    to: '/console/keys',
    cta: '管理密钥',
    accent: 'violet',
  },
  {
    iconId: 'chart',
    tag: '可观测',
    title: '用量、日志与导出',
    desc: '仪表盘按日/模型聚合消费，调用日志可追溯；支持一键导出 CSV / JSON 便于对账与内部分析。',
    to: '/console/logs',
    cta: '查看调用日志',
    accent: 'emerald',
  },
  {
    iconId: 'users',
    tag: '治理',
    title: '分组与倍率',
    desc: '用户归属分组，模型倍率与渠道路由在管理端可配，计费与配额策略更可预期。',
    to: '/console/group',
    cta: '我的分组',
    accent: 'sky',
  },
  {
    iconId: 'wallet',
    tag: '商业化',
    title: '充值与兑换码',
    desc: '余额与额度展示清晰；支持兑换码核销与充值入口，便于活动投放与团队发码。',
    to: '/console/billing',
    cta: '进入充值中心',
    accent: 'amber',
  },
  {
    iconId: 'shield',
    tag: '运营',
    title: '管理后台（RBAC）',
    desc: '用户与渠道、分组倍率、模型市场、用量与审计等能力面向 admin / superadmin，适合私有化交付与运维。',
    to: '/admin',
    cta: '管理后台',
    accent: 'fuchsia',
  },
]

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

const trustPoints = [
  { label: 'OpenAI 兼容协议', hint: '迁移成本低' },
  { label: 'Token 级计量', hint: '账单可对齐' },
  { label: '用量 CSV / JSON', hint: '便于审计' },
  { label: '渠道密钥 AES-GCM', hint: '存储加密' },
]

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

    <header class="th-site-header">
      <div class="mx-auto flex max-w-6xl items-center justify-between px-5 py-4">
        <div class="flex items-center gap-3">
          <div class="th-logo-mark th-logo-mark--sm shrink-0">TH</div>
          <div>
            <span class="th-sidebar-brand block text-lg font-semibold tracking-tight text-white">TraceHub</span>
            <span class="text-[10px] font-medium uppercase tracking-[0.18em] text-slate-500">AI Gateway</span>
          </div>
          <span class="ml-1 rounded border border-white/[0.08] bg-white/[0.04] px-2 py-0.5 font-mono text-[10px] text-slate-500">v{{ version }}</span>
        </div>
        <nav class="flex flex-wrap items-center justify-end gap-0.5 sm:gap-1">
          <a href="#features" class="th-site-nav-link">功能</a>
          <a href="#integration" class="th-site-nav-link hidden sm:inline">接入</a>
          <a href="#models" class="th-site-nav-link">模型</a>
          <a href="#pricing" class="th-site-nav-link">价格</a>
          <a href="#faq" class="th-site-nav-link">FAQ</a>
          <RouterLink to="/docs" class="th-site-nav-link">文档</RouterLink>
          <template v-if="!auth.isLoggedIn">
            <RouterLink to="/login" class="th-site-nav-link text-slate-300">登录</RouterLink>
            <RouterLink to="/register" class="th-btn-primary !px-4 !py-2 text-sm">注册</RouterLink>
          </template>
          <UserAccountMenu v-else accent="cyan" />
        </nav>
      </div>
    </header>

    <main class="relative z-[1]">
      <section class="relative mx-auto max-w-6xl overflow-hidden px-5 pb-24 pt-20 text-center md:pt-28">
        <div class="th-hero-backdrop" aria-hidden="true" />
        <div class="relative z-10 th-stagger-children">
        <p class="th-kicker mb-2"><span class="th-kicker-accent">TraceHub</span> · 企业级 AI API 网关</p>
        <h1 class="relative mx-auto max-w-4xl text-4xl font-semibold leading-[1.12] tracking-tight md:text-[3.25rem] md:leading-[1.1]">
          <span class="th-headline-gradient">统一接入</span>
          <span class="text-slate-200"> OpenAI · Claude · Gemini · DeepSeek</span>
        </h1>
        <p class="relative mx-auto mt-6 max-w-2xl text-base leading-relaxed text-slate-500 md:text-[1.0625rem] md:leading-relaxed">
          一套 OpenAI 兼容接口：渠道路由、Token 级计费、用量与审计可查，支持私有化部署与 SLA 扩展。
        </p>
        <div class="relative mt-10 flex flex-wrap justify-center gap-3 sm:gap-4">
          <RouterLink
            :to="auth.isLoggedIn ? '/console' : '/register'"
            class="th-btn-primary px-8 py-3.5 text-base sm:px-10"
          >
            {{ auth.isLoggedIn ? '进入控制台' : '立即开始' }}
          </RouterLink>
          <a href="#pricing" class="th-btn-secondary px-8 py-3.5 text-base sm:px-10">查看价格</a>
          <a href="#integration" class="th-btn-secondary px-8 py-3.5 text-base sm:px-10">接入示例</a>
        </div>
        <ul
          class="relative mx-auto mt-14 flex max-w-4xl flex-wrap justify-center gap-2 md:gap-3"
          aria-label="产品要点"
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
            不只是转发：控制台与治理能力
          </h2>
          <p class="mx-auto mt-3 max-w-2xl text-center text-sm leading-relaxed text-slate-500">
            从个人开发者到团队交付，TraceHub 在用户端与管理端覆盖了密钥、用量、计费与运维配置；以下为与仓库实现一致的核心能力展示。
          </p>
          <div class="mt-12 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <RouterLink
              v-for="f in features"
              :key="f.title"
              :to="featureLink(f.to)"
              class="group th-glass flex flex-col rounded-[var(--th-radius-xl)] p-6 no-underline transition duration-300 hover:bg-white/[0.04] hover:shadow-lg"
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
          <div class="grid gap-10 lg:grid-cols-2 lg:items-center">
            <div>
              <p class="th-kicker mb-2"><span class="th-kicker-accent">Integration</span></p>
              <h2 class="text-2xl font-semibold tracking-tight text-slate-100 md:text-[1.75rem]">与现有 OpenAI SDK 同一套调用方式</h2>
              <p class="mt-3 text-sm leading-relaxed text-slate-500">
                将 Base URL 指向 TraceHub 网关，使用控制台签发的 API Key 作为 Bearer Token，即可走统一计费与路由；流式场景下可沿用 SSE 解析逻辑。
              </p>
              <ul class="mt-6 space-y-3 text-sm text-slate-400">
                <li class="flex gap-2">
                  <span class="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-cyan-500/80" aria-hidden="true" />
                  <span><span class="font-mono text-xs text-cyan-200/90">GET /v1/models</span> 拉取当前账号可用模型（受分组与渠道配置影响）。</span>
                </li>
                <li class="flex gap-2">
                  <span class="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-cyan-500/80" aria-hidden="true" />
                  <span><span class="font-mono text-xs text-cyan-200/90">POST /v1/chat/completions</span> 对话补全；兼容渠道支持流式 chunk。</span>
                </li>
                <li class="flex gap-2">
                  <span class="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-cyan-500/80" aria-hidden="true" />
                  <span>生产环境建议配合 HTTPS、密钥轮换与 Prometheus <span class="font-mono text-xs text-slate-500">/metrics</span> 做健康与容量观测。</span>
                </li>
              </ul>
            </div>
            <div class="th-glass th-home-code-window overflow-hidden rounded-[var(--th-radius-xl)] p-1">
              <div class="th-home-code-window-titlebar flex items-center gap-2 border-b border-white/[0.06] bg-slate-950/80 px-4 py-2.5">
                <span class="h-2.5 w-2.5 rounded-full bg-red-500/70" aria-hidden="true" />
                <span class="h-2.5 w-2.5 rounded-full bg-amber-500/70" aria-hidden="true" />
                <span class="h-2.5 w-2.5 rounded-full bg-emerald-500/70" aria-hidden="true" />
                <span class="ml-2 font-mono text-[11px] text-slate-600">curl · chat.completions</span>
              </div>
              <pre class="th-home-code-window-pre max-h-[min(22rem,52vh)] overflow-auto bg-[#050a14] p-4 font-mono text-[11px] leading-relaxed text-slate-400 md:text-xs"><code><span class="th-home-code-comment text-slate-600"># 将 BASE 与 KEY 替换为你的网关地址与控制台 API Key</span>
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
          <h2 class="text-center text-2xl font-semibold tracking-tight text-slate-100 md:text-[1.75rem]">支持模型</h2>
          <p class="mx-auto mt-3 max-w-lg text-center text-sm leading-relaxed text-slate-500">
            以下为常见示例；实际上线模型与定价以控制台及管理员配置为准。
          </p>
          <div class="th-home-chip-row mt-10 flex flex-wrap justify-center gap-2">
            <span v-for="m in models" :key="m" class="th-chip">{{ m }}</span>
          </div>
        </div>
      </section>

      <section id="pricing" class="th-reveal mx-auto max-w-6xl px-5 py-20">
        <h2 class="text-center text-2xl font-semibold tracking-tight text-slate-100 md:text-[1.75rem]">透明计费</h2>
        <p class="mx-auto mt-3 max-w-lg text-center text-sm leading-relaxed text-slate-500">
          输入 / 输出按 1K Token 计价（额度单位）；分组与模型倍率在控制台可追溯。
        </p>
        <div class="th-table-wrap mt-10 overflow-hidden">
          <table class="w-full text-left text-sm">
            <thead>
              <tr class="border-b border-white/[0.06] bg-slate-950/50 text-[11px] font-semibold uppercase tracking-wider text-slate-500">
                <th class="px-5 py-3.5">模型</th>
                <th class="px-5 py-3.5">输入 / 1K Token</th>
                <th class="px-5 py-3.5">输出 / 1K Token</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-white/[0.05] text-slate-300">
              <tr class="transition hover:bg-white/[0.02]">
                <td class="px-5 py-4 font-mono text-xs text-slate-200">gpt-4o</td>
                <td class="px-5 py-4">300</td>
                <td class="px-5 py-4">1200</td>
              </tr>
              <tr class="transition hover:bg-white/[0.02]">
                <td class="px-5 py-4 font-mono text-xs text-slate-200">claude-3-5-sonnet</td>
                <td class="px-5 py-4">300</td>
                <td class="px-5 py-4">1500</td>
              </tr>
              <tr class="transition hover:bg-white/[0.02]">
                <td class="px-5 py-4 font-mono text-xs text-slate-200">gemini-2.0-flash</td>
                <td class="px-5 py-4">50</td>
                <td class="px-5 py-4">200</td>
              </tr>
              <tr class="transition hover:bg-white/[0.02]">
                <td class="px-5 py-4 font-mono text-xs text-slate-200">deepseek-chat</td>
                <td class="px-5 py-4">40</td>
                <td class="px-5 py-4">120</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p class="mt-6 text-center text-xs leading-relaxed text-slate-600">
          汇率与额度换算可在部署配置中调整；管理员在后台维护分组、渠道与模型倍率。
        </p>
      </section>

      <section id="faq" class="th-reveal border-t border-white/[0.06] py-20">
        <div class="mx-auto max-w-3xl px-5">
          <h2 class="text-center text-2xl font-semibold tracking-tight text-slate-100 md:text-[1.75rem]">常见问题</h2>
          <div class="mt-10 space-y-3">
            <div v-for="(f, i) in faqs" :key="i" class="th-glass th-faq-item rounded-[var(--th-radius-lg)] p-6">
              <p class="text-[15px] font-medium leading-snug text-slate-200">{{ f.q }}</p>
              <p class="mt-3 text-sm leading-relaxed text-slate-500">{{ f.a }}</p>
            </div>
          </div>
        </div>
      </section>
    </main>

    <footer class="relative z-[1] border-t border-white/[0.06] py-14">
      <div class="mx-auto flex max-w-6xl flex-col gap-10 px-5 md:flex-row md:justify-between md:gap-12">
        <div class="max-w-sm">
          <div class="flex items-center gap-2">
            <div class="th-logo-mark th-logo-mark--sm shrink-0">TH</div>
            <span class="th-sidebar-brand text-base font-semibold text-slate-200">TraceHub</span>
          </div>
          <p class="mt-3 text-sm leading-relaxed text-slate-500">
            开源 AI API 网关：统一 OpenAI 兼容入口、计量计费与用户控制台，可按仓库文档私有化部署。
          </p>
        </div>
        <div class="grid grid-cols-2 gap-10 sm:grid-cols-3 md:gap-14">
          <div>
            <p class="text-[11px] font-semibold uppercase tracking-widest text-slate-600">产品</p>
            <ul class="mt-3 space-y-2 text-sm text-slate-400">
              <li><a href="#features" class="transition hover:text-cyan-400/90">功能亮点</a></li>
              <li><a href="#integration" class="transition hover:text-cyan-400/90">接入说明</a></li>
              <li><a href="#models" class="transition hover:text-cyan-400/90">支持模型</a></li>
              <li><a href="#pricing" class="transition hover:text-cyan-400/90">计费说明</a></li>
            </ul>
          </div>
          <div>
            <p class="text-[11px] font-semibold uppercase tracking-widest text-slate-600">账户</p>
            <ul class="mt-3 space-y-2 text-sm text-slate-400">
              <li><RouterLink to="/register" class="transition hover:text-cyan-400/90">注册</RouterLink></li>
              <li><RouterLink to="/login" class="transition hover:text-cyan-400/90">登录</RouterLink></li>
              <li><RouterLink to="/console" class="transition hover:text-cyan-400/90">用户控制台</RouterLink></li>
            </ul>
          </div>
          <div class="col-span-2 sm:col-span-1">
            <p class="text-[11px] font-semibold uppercase tracking-widest text-slate-600">文档</p>
            <ul class="mt-3 space-y-2 text-sm text-slate-400">
              <li>
                <span class="leading-relaxed">产品需求与接口细节见仓库内 </span>
                <code class="rounded bg-white/[0.06] px-1.5 py-0.5 font-mono text-[11px] text-slate-500">docs/</code>
                <span class="leading-relaxed"> 目录（随部署版本更新）。</span>
              </li>
            </ul>
          </div>
        </div>
      </div>
      <p class="mx-auto mt-10 max-w-6xl border-t border-white/[0.05] px-5 pt-8 text-center text-xs leading-relaxed text-slate-600">
        生产环境请配置 HTTPS、密钥轮换与观测告警；Prometheus 抓取 <span class="font-mono text-slate-500">/metrics</span>，健康检查
        <span class="font-mono text-slate-500">/health</span>、<span class="font-mono text-slate-500">/ready</span>。
      </p>
    </footer>
  </div>
</template>
