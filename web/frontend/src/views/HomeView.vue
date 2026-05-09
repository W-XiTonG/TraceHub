<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useAuthStore } from '../stores/auth'

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
  <div class="min-h-screen bg-slate-950 text-slate-100">
    <header class="border-b border-slate-800 bg-slate-950/80 backdrop-blur sticky top-0 z-20">
      <div class="mx-auto flex max-w-6xl items-center justify-between px-4 py-4">
        <div class="flex items-center gap-2">
          <span class="text-xl font-bold tracking-tight text-cyan-400">TraceHub</span>
          <span class="rounded bg-slate-800 px-2 py-0.5 text-xs text-slate-400">v{{ version }}</span>
        </div>
        <nav class="flex items-center gap-4 text-sm">
          <a href="#models" class="text-slate-400 hover:text-white">模型</a>
          <a href="#pricing" class="text-slate-400 hover:text-white">价格</a>
          <a href="#faq" class="text-slate-400 hover:text-white">FAQ</a>
          <RouterLink v-if="!auth.isLoggedIn" to="/login" class="text-slate-300 hover:text-white">登录</RouterLink>
          <RouterLink v-if="!auth.isLoggedIn" to="/register" class="rounded-lg bg-cyan-500 px-3 py-1.5 font-medium text-slate-950 hover:bg-cyan-400">注册</RouterLink>
          <RouterLink v-if="auth.isLoggedIn" to="/console" class="text-cyan-400 hover:text-cyan-300">控制台</RouterLink>
          <RouterLink v-if="auth.role === 'superadmin' || auth.role === 'admin'" to="/admin" class="text-amber-400 hover:text-amber-300">管理后台</RouterLink>
        </nav>
      </div>
    </header>

    <main>
      <section class="mx-auto max-w-6xl px-4 py-20 text-center">
        <p class="mb-4 text-sm uppercase tracking-[0.3em] text-cyan-500/90">AI API 网关</p>
        <h1 class="mb-6 text-4xl font-bold leading-tight md:text-6xl">
          统一接入 OpenAI / Claude / Gemini / DeepSeek
        </h1>
        <p class="mx-auto mb-10 max-w-2xl text-lg text-slate-400">
          一套 OpenAI 兼容接口，智能路由、Token 级计费、多账号负载与粘性会话，可私有化部署。
        </p>
        <div class="flex flex-wrap justify-center gap-4">
          <RouterLink to="/register" class="rounded-xl bg-cyan-500 px-8 py-3 font-semibold text-slate-950 hover:bg-cyan-400">立即开始</RouterLink>
          <a href="#pricing" class="rounded-xl border border-slate-600 px-8 py-3 font-semibold text-slate-200 hover:border-slate-400">查看价格</a>
        </div>
      </section>

      <section id="models" class="border-y border-slate-800 bg-slate-900/40 py-16">
        <div class="mx-auto max-w-6xl px-4">
          <h2 class="mb-8 text-center text-2xl font-semibold">支持模型（示例）</h2>
          <div class="flex flex-wrap justify-center gap-2">
            <span v-for="m in models" :key="m" class="rounded-full border border-slate-700 bg-slate-800/80 px-4 py-1.5 text-sm text-slate-300">{{ m }}</span>
          </div>
          <p class="mt-6 text-center text-sm text-slate-500">实际可用模型以控制台绑定渠道为准。</p>
        </div>
      </section>

      <section id="pricing" class="mx-auto max-w-6xl px-4 py-16">
        <h2 class="mb-8 text-center text-2xl font-semibold">透明计费</h2>
        <div class="overflow-hidden rounded-2xl border border-slate-800">
          <table class="w-full text-left text-sm">
            <thead class="bg-slate-900 text-slate-400">
              <tr>
                <th class="px-4 py-3">模型</th>
                <th class="px-4 py-3">输入 / 1K Token（额度）</th>
                <th class="px-4 py-3">输出 / 1K Token（额度）</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-800 bg-slate-950">
              <tr><td class="px-4 py-3">gpt-4o</td><td class="px-4 py-3">300</td><td class="px-4 py-3">1200</td></tr>
              <tr><td class="px-4 py-3">claude-3-5-sonnet</td><td class="px-4 py-3">300</td><td class="px-4 py-3">1500</td></tr>
              <tr><td class="px-4 py-3">gemini-2.0-flash</td><td class="px-4 py-3">50</td><td class="px-4 py-3">200</td></tr>
              <tr><td class="px-4 py-3">deepseek-chat</td><td class="px-4 py-3">40</td><td class="px-4 py-3">120</td></tr>
            </tbody>
          </table>
        </div>
        <p class="mt-4 text-center text-xs text-slate-500">分组倍率、模型倍率可在管理后台调整；1 元人民币默认对应 10000 额度（可配置）。</p>
      </section>

      <section id="faq" class="border-t border-slate-800 bg-slate-900/30 py-16">
        <div class="mx-auto max-w-3xl px-4">
          <h2 class="mb-8 text-center text-2xl font-semibold">常见问题</h2>
          <div class="space-y-4">
            <div v-for="(f, i) in faqs" :key="i" class="rounded-xl border border-slate-800 bg-slate-950 p-4">
              <p class="font-medium text-cyan-400">{{ f.q }}</p>
              <p class="mt-2 text-sm text-slate-400">{{ f.a }}</p>
            </div>
          </div>
        </div>
      </section>
    </main>

    <footer class="border-t border-slate-800 py-8 text-center text-xs text-slate-600">
      <p>TraceHub · 文档与 OpenAPI 可参考仓库内 docs · 更新时间 2026-05-09</p>
      <p class="mt-2">联系支持：请在管理后台配置 SMTP 与站点信息后启用邮件工单。</p>
    </footer>
  </div>
</template>
