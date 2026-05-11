<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import UserAccountMenu from '../components/UserAccountMenu.vue'

const auth = useAuthStore()
const section = ref<'guide' | 'api'>('guide')
</script>

<template>
  <div class="th-page-shell min-h-screen text-slate-100">
    <header class="th-site-header">
      <div class="mx-auto flex max-w-5xl items-center justify-between px-5 py-4">
        <RouterLink to="/" class="flex items-center gap-3 transition hover:opacity-90">
          <div class="th-logo-mark th-logo-mark--sm shrink-0">TH</div>
          <div>
            <span class="th-sidebar-brand block text-lg font-semibold tracking-tight text-white">TraceHub</span>
            <span class="text-[10px] font-medium uppercase tracking-[0.18em] text-slate-500">文档</span>
          </div>
        </RouterLink>
        <nav class="flex flex-wrap items-center justify-end gap-1 sm:gap-2">
          <RouterLink to="/" class="th-site-nav-link">首页</RouterLink>
          <RouterLink
            :to="auth.isLoggedIn ? '/console' : '/login'"
            class="th-site-nav-link"
          >
            {{ auth.isLoggedIn ? '控制台' : '登录' }}
          </RouterLink>
          <UserAccountMenu v-if="auth.isLoggedIn" accent="cyan" />
        </nav>
      </div>
    </header>

    <main class="mx-auto max-w-5xl px-5 py-10 pb-20 md:py-14">
      <p class="th-kicker mb-2"><span class="th-kicker-accent">Docs</span></p>
      <h1 class="text-3xl font-semibold tracking-tight text-slate-100 md:text-4xl">使用与 API 文档</h1>
      <p class="mt-3 max-w-2xl text-sm leading-relaxed text-slate-500">
        以下为 TraceHub 常用操作说明与对外接口摘要；完整设计见仓库内 Markdown。
      </p>

      <div class="mt-8 flex flex-wrap gap-2 border-b border-white/[0.08] pb-4">
        <button
          type="button"
          class="th-pill"
          :class="{ 'th-pill-active': section === 'guide' }"
          @click="section = 'guide'"
        >
          操作指南
        </button>
        <button
          type="button"
          class="th-pill"
          :class="{ 'th-pill-active': section === 'api' }"
          @click="section = 'api'"
        >
          API 参考
        </button>
      </div>

      <!-- 操作指南 -->
      <article v-show="section === 'guide'" class="docs-prose mt-8 space-y-8">
        <section class="th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
          <h2 class="text-lg font-semibold text-slate-100">TraceHub 是什么</h2>
          <p class="mt-3 text-sm leading-relaxed text-slate-400">
            TraceHub 是 AI API 网关：将多种上游以 <strong class="text-slate-300">OpenAI 兼容</strong> 方式统一暴露，并提供用户、API Key、分组与计费、控制台与管理后台。
          </p>
        </section>

        <section class="th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
          <h2 class="text-lg font-semibold text-slate-100">快速上手</h2>
          <ol class="mt-4 list-decimal space-y-3 pl-5 text-sm leading-relaxed text-slate-400">
            <li>部署或启动后端（默认监听配置中的地址，如 <code class="docs-code">:8080</code>），并配置 <code class="docs-code">configs/config.yaml</code>（数据库、JWT 等）。</li>
            <li>在官网 <RouterLink to="/register" class="th-link-accent underline-offset-2 hover:underline">注册</RouterLink> 账号，或管理员在库中创建用户。</li>
            <li>登录后进入 <strong class="text-slate-300">控制台</strong>，在「API Key」中新建密钥并妥善保存（完整 Key 仅创建时显示一次）。</li>
            <li>调用网关时在请求头携带 <code class="docs-code">Authorization: Bearer &lt;你的 API Key&gt;</code>。</li>
          </ol>
        </section>

        <section class="th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
          <h2 class="text-lg font-semibold text-slate-100">控制台常用功能</h2>
          <ul class="mt-4 space-y-2 text-sm text-slate-400">
            <li><strong class="text-slate-300">概览</strong>：余额、用量周期、导出 CSV/JSON。</li>
            <li><strong class="text-slate-300">可用模型</strong>：当前分组下可调用模型列表（与 <code class="docs-code">GET /v1/models</code> 一致来源说明见页内）。</li>
            <li><strong class="text-slate-300">API Key</strong>：创建、编辑、IP 白名单、启用/禁用。</li>
            <li><strong class="text-slate-300">调用日志</strong>：按时间、模型、状态码筛选。</li>
            <li><strong class="text-slate-300">我的分组</strong>：查看所属分组与倍率等信息。</li>
            <li><strong class="text-slate-300">充值 / 兑换码</strong>：按部署配置使用。</li>
            <li><strong class="text-slate-300">账户设置</strong>：密码、昵称、Webhook、账号级 IP 白名单等。</li>
          </ul>
          <p v-if="auth.role === 'admin' || auth.role === 'superadmin'" class="mt-4 text-sm text-amber-200/85">
            管理员还可进入 <RouterLink to="/admin" class="underline-offset-2 hover:underline">管理后台</RouterLink> 维护用户、渠道、分组、模型市场等。
          </p>
        </section>

        <section class="th-glass--subtle rounded-[var(--th-radius-lg)] p-5 text-xs leading-relaxed text-slate-500">
          仓库内更完整说明：<span class="font-mono text-slate-400">README.zh-CN.md</span>、<span class="font-mono text-slate-400">docs/产品文档.md</span>、<span class="font-mono text-slate-400">docs/development-docs/deployment-ops.md</span>（部署与运维）。
        </section>
      </article>

      <!-- API 参考 -->
      <article v-show="section === 'api'" class="docs-prose mt-8 space-y-8">
        <section class="th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
          <h2 class="text-lg font-semibold text-slate-100">网关（对外 AI）</h2>
          <p class="mt-3 text-sm text-slate-400">Base 与部署域名一致，路径前缀 <code class="docs-code">/v1</code>。认证：</p>
          <pre class="docs-pre mt-3">Authorization: Bearer &lt;API Key&gt;</pre>
          <div class="mt-6 space-y-4 text-sm text-slate-400">
            <div>
              <p class="font-mono text-xs text-cyan-200/90">POST /v1/chat/completions</p>
              <p class="mt-1">OpenAI 兼容聊天补全；OpenAI/DeepSeek 等渠道支持 SSE 流式（<code class="docs-code">stream: true</code>）。</p>
            </div>
            <div>
              <p class="font-mono text-xs text-cyan-200/90">GET /v1/models</p>
              <p class="mt-1">列出当前 Key 所属分组下可用模型（与控制台「可用模型」一致逻辑）。</p>
            </div>
          </div>
        </section>

        <section class="th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
          <h2 class="text-lg font-semibold text-slate-100">用户 JSON API（需登录 JWT）</h2>
          <p class="mt-3 text-sm text-slate-400">前缀 <code class="docs-code">/api/user</code>，除注册/登录/刷新外需在 Header 携带 <code class="docs-code">Authorization: Bearer &lt;access_token&gt;</code>。</p>
          <ul class="mt-4 space-y-2 font-mono text-xs text-slate-300">
            <li><span class="text-emerald-400/90">POST</span> /api/user/register</li>
            <li><span class="text-emerald-400/90">POST</span> /api/user/login</li>
            <li><span class="text-emerald-400/90">POST</span> /api/user/refresh</li>
            <li><span class="text-sky-400/90">GET</span> /api/user/me</li>
            <li><span class="text-sky-400/90">GET</span> /api/user/balance</li>
            <li><span class="text-sky-400/90">GET</span> /api/user/usage-logs</li>
            <li><span class="text-sky-400/90">GET</span> /api/user/group</li>
            <li><span class="text-sky-400/90">GET</span> /api/user/available-models、GET /api/user/models</li>
            <li><span class="text-sky-400/90">GET</span> /api/user/usage、GET /api/user/usage/export</li>
            <li><span class="text-sky-400/90">GET</span> /api/user/apikeys · <span class="text-amber-400/90">POST</span> /api/user/apikeys · <span class="text-amber-400/90">PUT</span> /api/user/apikeys/:id · <span class="text-rose-400/90">DELETE</span> /api/user/apikeys/:id</li>
            <li><span class="text-amber-400/90">POST</span> /api/user/redeem</li>
            <li><span class="text-amber-400/90">PUT</span> /api/user/password、/api/user/bind、/api/user/ip_whitelist</li>
          </ul>
        </section>

        <section class="th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
          <h2 class="text-lg font-semibold text-slate-100">管理端 JSON API</h2>
          <p class="mt-3 text-sm text-slate-400">前缀 <code class="docs-code">/api/admin</code>，需 JWT 且角色为 <code class="docs-code">admin</code> 或 <code class="docs-code">superadmin</code>。</p>
          <p class="mt-3 text-sm text-slate-400">包含仪表盘摘要、用户与余额、渠道 CRUD、分组与模型倍率、日志与导出、兑换码、系统配置、模型市场来源与条目等。详见仓库路由与 <span class="font-mono text-slate-500">internal/server/router.go</span>。</p>
        </section>

        <section class="th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
          <h2 class="text-lg font-semibold text-slate-100">健康与观测</h2>
          <ul class="mt-3 font-mono text-xs text-slate-300">
            <li><span class="text-sky-400/90">GET</span> /health</li>
            <li><span class="text-sky-400/90">GET</span> /ready</li>
            <li><span class="text-sky-400/90">GET</span> /metrics（Prometheus）</li>
          </ul>
        </section>

        <section class="th-glass--subtle rounded-[var(--th-radius-lg)] p-5 text-xs leading-relaxed text-slate-500">
          完整接口说明、请求体字段与示例见仓库：<span class="font-mono text-slate-400">docs/development-docs/api-design.md</span>。
        </section>
      </article>
    </main>

    <footer class="border-t border-white/[0.06] py-10 text-center">
      <RouterLink to="/" class="text-sm text-slate-500 transition hover:text-slate-400">返回首页</RouterLink>
    </footer>
  </div>
</template>

<style scoped>
.docs-code {
  border-radius: 0.25rem;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(0, 0, 0, 0.35);
  padding: 0.1rem 0.4rem;
  font-family: var(--th-font-mono);
  font-size: 0.75rem;
  color: rgb(165, 243, 252);
}
.docs-pre {
  overflow-x: auto;
  border-radius: 0.5rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(2, 6, 23, 0.65);
  padding: 0.875rem 1rem;
  font-family: var(--th-font-mono);
  font-size: 0.75rem;
  line-height: 1.5;
  color: rgb(203, 213, 225);
}
</style>
