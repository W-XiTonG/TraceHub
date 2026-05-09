<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import UserAccountMenu from '../components/UserAccountMenu.vue'

const auth = useAuthStore()
const version = ref('1.0.0')

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
  <div class="th-page-shell text-slate-100">
    <header class="th-site-header">
      <div class="mx-auto flex max-w-6xl items-center justify-between px-5 py-4">
        <div class="flex items-center gap-3">
          <div class="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-br from-cyan-400 to-teal-600 font-mono text-sm font-bold text-slate-950 shadow-lg shadow-cyan-500/25">TH</div>
          <div>
            <span class="th-sidebar-brand block text-lg font-semibold tracking-tight text-white">TraceHub</span>
            <span class="text-[10px] font-medium uppercase tracking-widest text-slate-500">AI Gateway</span>
          </div>
          <span class="ml-1 rounded-md border border-white/10 bg-white/5 px-2 py-0.5 font-mono text-[10px] text-slate-400">v{{ version }}</span>
        </div>
        <nav class="flex items-center gap-1 sm:gap-2">
          <a href="#models" class="rounded-lg px-3 py-2 text-sm text-slate-400 transition hover:bg-white/5 hover:text-white">模型</a>
          <a href="#pricing" class="rounded-lg px-3 py-2 text-sm text-slate-400 transition hover:bg-white/5 hover:text-white">价格</a>
          <a href="#faq" class="rounded-lg px-3 py-2 text-sm text-slate-400 transition hover:bg-white/5 hover:text-white">FAQ</a>
          <template v-if="!auth.isLoggedIn">
            <RouterLink to="/login" class="rounded-lg px-3 py-2 text-sm text-slate-300 transition hover:bg-white/5 hover:text-white">登录</RouterLink>
            <RouterLink to="/register" class="th-btn-primary !px-4 !py-2 text-sm">注册</RouterLink>
          </template>
          <UserAccountMenu v-else accent="cyan" />
        </nav>
      </div>
    </header>

    <main>
      <section class="relative mx-auto max-w-6xl px-5 pb-24 pt-20 text-center md:pt-28">
        <div class="pointer-events-none absolute left-1/2 top-0 h-64 w-[120%] -translate-x-1/2 bg-[radial-gradient(ellipse_at_center,rgba(34,211,238,0.15),transparent_70%)]" />
        <p class="th-kicker relative mb-5">企业级 AI API 网关</p>
        <h1 class="relative mx-auto max-w-4xl text-4xl font-bold leading-[1.1] tracking-tight md:text-6xl md:leading-[1.08]">
          <span class="th-headline-gradient">统一接入</span>
          <span class="text-slate-100"> OpenAI · Claude · Gemini · DeepSeek</span>
        </h1>
        <p class="relative mx-auto mt-6 max-w-2xl text-base leading-relaxed text-slate-400 md:text-lg">
          一套 OpenAI 兼容接口，智能路由、Token 级计费、多账号负载与粘性会话，可私有化部署。
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
      </section>

      <section id="models" class="relative border-y border-white/[0.06] py-20">
        <div class="absolute inset-0 bg-gradient-to-b from-transparent via-slate-900/30 to-transparent" />
        <div class="relative mx-auto max-w-6xl px-5">
          <h2 class="text-center text-2xl font-semibold tracking-tight text-white md:text-3xl">支持模型</h2>
          <p class="mx-auto mt-2 max-w-lg text-center text-sm text-slate-500">以下为常见示例，实际上线模型以渠道配置为准。</p>
          <div class="mt-10 flex flex-wrap justify-center gap-2.5">
            <span
              v-for="m in models"
              :key="m"
              class="rounded-full border border-cyan-500/20 bg-gradient-to-b from-cyan-950/40 to-slate-900/60 px-4 py-2 font-mono text-xs text-cyan-100/90 shadow-sm shadow-black/20 backdrop-blur-sm transition hover:border-cyan-400/35 hover:from-cyan-900/50 md:text-sm"
            >
              {{ m }}
            </span>
          </div>
        </div>
      </section>

      <section id="pricing" class="mx-auto max-w-6xl px-5 py-20">
        <h2 class="text-center text-2xl font-semibold tracking-tight text-white md:text-3xl">透明计费</h2>
        <p class="mx-auto mt-2 max-w-lg text-center text-sm text-slate-500">输入 / 输出按 1K Token 计价（额度单位），分组与模型倍率可叠加。</p>
        <div class="th-table-wrap mt-10 overflow-hidden">
          <table class="w-full text-left text-sm">
            <thead>
              <tr class="border-b border-white/[0.06] bg-slate-900/80 text-xs uppercase tracking-wider text-slate-500">
                <th class="px-5 py-4 font-medium">模型</th>
                <th class="px-5 py-4 font-medium">输入 / 1K Token</th>
                <th class="px-5 py-4 font-medium">输出 / 1K Token</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-white/[0.05] text-slate-300">
              <tr class="transition hover:bg-white/[0.02]">
                <td class="px-5 py-4 font-mono text-xs text-cyan-200/90">gpt-4o</td>
                <td class="px-5 py-4">300</td>
                <td class="px-5 py-4">1200</td>
              </tr>
              <tr class="transition hover:bg-white/[0.02]">
                <td class="px-5 py-4 font-mono text-xs text-cyan-200/90">claude-3-5-sonnet</td>
                <td class="px-5 py-4">300</td>
                <td class="px-5 py-4">1500</td>
              </tr>
              <tr class="transition hover:bg-white/[0.02]">
                <td class="px-5 py-4 font-mono text-xs text-cyan-200/90">gemini-2.0-flash</td>
                <td class="px-5 py-4">50</td>
                <td class="px-5 py-4">200</td>
              </tr>
              <tr class="transition hover:bg-white/[0.02]">
                <td class="px-5 py-4 font-mono text-xs text-cyan-200/90">deepseek-chat</td>
                <td class="px-5 py-4">40</td>
                <td class="px-5 py-4">120</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p class="mt-6 text-center text-xs leading-relaxed text-slate-600">1 元人民币默认对应 10000 额度（可配置）；管理员可在后台调整分组与模型倍率。</p>
      </section>

      <section id="faq" class="border-t border-white/[0.06] py-20">
        <div class="mx-auto max-w-3xl px-5">
          <h2 class="text-center text-2xl font-semibold tracking-tight text-white md:text-3xl">常见问题</h2>
          <div class="mt-10 space-y-4">
            <div
              v-for="(f, i) in faqs"
              :key="i"
              class="th-glass group p-6 transition hover:border-cyan-500/20"
            >
              <p class="font-medium text-cyan-300/95 transition group-hover:text-cyan-200">{{ f.q }}</p>
              <p class="mt-3 text-sm leading-relaxed text-slate-400">{{ f.a }}</p>
            </div>
          </div>
        </div>
      </section>
    </main>

    <footer class="border-t border-white/[0.06] py-12 text-center">
      <p class="text-xs text-slate-600">TraceHub · 文档见仓库 docs · 2026-05-09</p>
      <p class="mt-2 text-xs text-slate-700">企业部署与 SLA 支持请联系管理员配置 SMTP 与站点信息。</p>
    </footer>
  </div>
</template>
