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
    a: 'OpenAI 兼容渠道支持 SSE 流式；Claude / Gemini 渠道当前默认非流式（网关可持续演进）。',
  },
  {
    q: '数据安全如何保障？',
    a: '上游密钥 AES-GCM 加密存储，API Key 仅保存哈希；支持私有化部署与审计日志扩展。',
  },
]
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
        <nav class="flex items-center gap-0.5 sm:gap-1">
          <a href="#models" class="th-site-nav-link">模型</a>
          <a href="#pricing" class="th-site-nav-link">价格</a>
          <a href="#faq" class="th-site-nav-link">FAQ</a>
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
        <div class="relative mt-10 flex flex-wrap justify-center gap-4">
          <RouterLink
            :to="auth.isLoggedIn ? '/console' : '/login'"
            class="th-btn-primary px-10 py-3.5 text-base"
          >
            {{ auth.isLoggedIn ? '进入控制台' : '立即开始' }}
          </RouterLink>
          <a href="#pricing" class="th-btn-secondary px-10 py-3.5 text-base">查看价格</a>
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

    <footer class="relative z-[1] border-t border-white/[0.06] py-12 text-center">
      <p class="text-xs font-medium text-slate-500">TraceHub</p>
      <p class="mx-auto mt-2 max-w-md text-xs leading-relaxed text-slate-600">
        产品与接口说明见仓库文档；生产环境请配置 HTTPS、密钥轮换与审计策略。
      </p>
    </footer>
  </div>
</template>
