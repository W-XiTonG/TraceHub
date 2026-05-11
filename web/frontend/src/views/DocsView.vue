<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { RouterLink } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import UserAccountMenu from '../components/UserAccountMenu.vue'

const auth = useAuthStore()
const activeId = ref('doc-intro')
const mobileTocOpen = ref(false)

const toc = [
  { id: 'doc-intro', label: '产品简介' },
  { id: 'doc-concepts', label: '核心概念' },
  { id: 'doc-quickstart', label: '快速开始' },
  { id: 'doc-console', label: '控制台说明' },
  { id: 'doc-auth', label: '认证与安全' },
  { id: 'doc-gateway', label: '网关 /v1（OpenAI 兼容）' },
  { id: 'doc-headers', label: '选路与会话头' },
  { id: 'doc-user-api', label: '用户 JSON API' },
  { id: 'doc-admin-api', label: '管理端 API' },
  { id: 'doc-ops', label: '健康与观测' },
  { id: 'doc-errors', label: '错误与排查' },
  { id: 'doc-faq', label: '常见问题' },
] as const

let observer: IntersectionObserver | null = null

function scrollToSection(id: string) {
  activeId.value = id
  mobileTocOpen.value = false
  const el = document.getElementById(id)
  el?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

onMounted(() => {
  observer = new IntersectionObserver(
    (entries) => {
      const visible = entries.filter((e) => e.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)
      if (visible.length) {
        activeId.value = visible[0].target.id
      }
    },
    { root: null, rootMargin: '-72px 0px -55% 0px', threshold: [0, 0.01] },
  )
  toc.forEach(({ id }) => {
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
    <header class="th-site-header sticky top-0 z-30 border-b border-white/[0.06] backdrop-blur-md">
      <div class="mx-auto flex max-w-6xl items-center justify-between gap-4 px-4 py-3.5 sm:px-5">
        <RouterLink to="/" class="flex min-w-0 items-center gap-3 transition hover:opacity-90">
          <div class="th-logo-mark th-logo-mark--sm shrink-0">TH</div>
          <div class="min-w-0">
            <span class="th-sidebar-brand block truncate text-lg font-semibold tracking-tight text-white">TraceHub</span>
            <span class="text-[10px] font-medium uppercase tracking-[0.18em] text-slate-500">文档中心</span>
          </div>
        </RouterLink>
        <nav class="flex shrink-0 flex-wrap items-center justify-end gap-1 sm:gap-2">
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

    <div class="mx-auto grid max-w-6xl gap-8 px-4 pb-20 pt-8 lg:grid-cols-[13.5rem_minmax(0,1fr)] lg:gap-12 lg:px-5 lg:pt-10">
      <!-- 移动端目录 -->
      <div class="lg:hidden">
        <button
          type="button"
          class="docs-mobile-toc-btn flex w-full items-center justify-between rounded-xl border border-white/[0.1] bg-white/[0.04] px-4 py-3 text-left text-sm font-medium text-slate-200"
          :aria-expanded="mobileTocOpen"
          @click="mobileTocOpen = !mobileTocOpen"
        >
          <span>本页目录</span>
          <svg class="h-4 w-4 text-slate-500 transition" :class="{ 'rotate-180': mobileTocOpen }" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
            <path fill-rule="evenodd" d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.94a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z" clip-rule="evenodd" />
          </svg>
        </button>
        <nav v-show="mobileTocOpen" class="mt-2 space-y-0.5 rounded-xl border border-white/[0.08] bg-slate-950/90 p-2">
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
          <p class="mb-3 text-[10px] font-semibold uppercase tracking-[0.2em] text-slate-600">目录</p>
          <nav class="space-y-0.5 border-l border-white/[0.08] pl-3">
            <button
              v-for="item in toc"
              :key="item.id"
              type="button"
              class="docs-toc-link -ml-px block w-full border-l-2 border-transparent py-1.5 pl-3 text-left text-xs leading-snug text-slate-500 transition hover:text-slate-300"
              :class="{ 'docs-toc-link--active !border-cyan-500/70 !text-cyan-200/95': activeId === item.id }"
              @click="scrollToSection(item.id)"
            >
              {{ item.label }}
            </button>
          </nav>
        </div>
      </aside>

      <main class="docs-main min-w-0">
        <p class="th-kicker mb-2"><span class="th-kicker-accent">Documentation</span></p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-100 md:text-4xl">TraceHub 使用与 API 文档</h1>
        <p class="mt-3 max-w-2xl text-sm leading-relaxed text-slate-500">
          面向最终用户与集成方：说明产品能力、控制台操作、OpenAI 兼容网关及配套 REST 接口。部署细节与仓库内设计文档互为补充。
        </p>

        <article class="docs-prose mt-10 space-y-12">
          <section id="doc-intro" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>产品简介</h2>
            <p>
              <strong class="text-slate-200">TraceHub</strong> 是一类 <strong class="text-slate-200">AI API 网关</strong>：在后台接入多种上游渠道（OpenAI 形态、DeepSeek、Gemini 等，具体以管理员配置为准），对调用方统一暴露
              <strong class="text-slate-200">OpenAI 兼容的 <code class="docs-code">/v1</code> HTTP API</strong>，并在平台侧完成
              <strong class="text-slate-200">用户、API Key、分组、模型可见性、计费与用量日志</strong> 等能力。您可以使用官方 OpenAI SDK、LangChain、自研 HTTP 客户端等，只需把 Base URL 与密钥换成 TraceHub 提供的即可。
            </p>
            <p>本页为对外文档中心；更完整的接口字段与运维说明仍可在仓库的 <code class="docs-code">docs/development-docs/api-design.md</code> 等 Markdown 中查阅。</p>
          </section>

          <section id="doc-concepts" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>核心概念</h2>
            <dl class="docs-dl mt-4">
              <dt>用户与 JWT</dt>
              <dd>在官网注册后，您用邮箱密码登录 Web 控制台；浏览器会话使用 JWT（<code class="docs-code">access_token</code> / <code class="docs-code">refresh_token</code>）。调用 <code class="docs-code">/api/user/*</code> 时使用 JWT，而非 API Key。</dd>
              <dt>API Key</dt>
              <dd>用于调用 <strong class="text-slate-300">网关 <code class="docs-code">/v1/*</code></strong>。可在控制台创建多个 Key，分别设置备注、IP 白名单与启用状态。完整密钥仅在<strong class="text-amber-200/90">创建成功时展示一次</strong>，请立即保存。</dd>
              <dt>分组</dt>
              <dd>每个用户属于一个分组。分组决定<strong class="text-slate-300">可用模型列表</strong>、渠道选路与计费倍率等。未分配分组时，网关可能返回权限类错误，需联系管理员。</dd>
              <dt>渠道与上游</dt>
              <dd>管理员在后台配置「渠道」（上游 Base URL、鉴权方式、类型等）。网关根据模型名与策略为每次请求选择渠道并转发。</dd>
              <dt>余额与计费</dt>
              <dd>按平台规则从账户余额扣费（具体换算以部署配置为准，例如配置项 <code class="docs-code">site.currency_quota_per_yuan</code>）。余额不足时，网关可能返回 HTTP <code class="docs-code">402</code>。</dd>
            </dl>
          </section>

          <section id="doc-quickstart" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>快速开始</h2>
            <ol class="docs-ol mt-4">
              <li>确认后端已启动（默认示例为监听 <code class="docs-code">configs/config.yaml</code> 中 <code class="docs-code">server.addr</code>，如 <code class="docs-code">:8080</code>），数据库与 Redis 可用。</li>
              <li>在 <RouterLink to="/register" class="th-link-accent">注册</RouterLink> 页面创建账号，或使用管理员预置账号（见配置 <code class="docs-code">bootstrap</code>）。</li>
              <li><RouterLink to="/login" class="th-link-accent">登录</RouterLink> 后进入控制台，在「API Key」中新建密钥并复制保存。</li>
              <li>将应用中的 Base URL 设为<strong class="text-slate-300">网关根地址</strong>（与浏览器访问站点同源即可，路径前缀为 <code class="docs-code">/v1</code>），请求头增加 <code class="docs-code">Authorization: Bearer &lt;API Key&gt;</code>。</li>
            </ol>
            <p class="docs-callout mt-6">
              <strong class="text-slate-200">最小 cURL 示例（聊天）</strong> — 将 <code class="docs-code">$BASE</code> 换成您的服务根（如 <code class="docs-code">http://localhost:8080</code>），<code class="docs-code">$KEY</code> 换成 API Key。
            </p>
            <pre class="docs-pre mt-3">curl -sS "$BASE/v1/chat/completions" \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role":"user","content":"用一句话介绍 TraceHub"}],
    "stream": false
  }'</pre>
            <p class="mt-4 text-sm text-slate-500">流式响应时设置 <code class="docs-code">"stream": true</code>，按 SSE 解析数据行（与 OpenAI 一致，末行为 <code class="docs-code">data: [DONE]</code>）。</p>
          </section>

          <section id="doc-console" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>控制台说明</h2>
            <p>登录后左侧导航各模块用途如下（文案以界面为准）。</p>
            <ul class="docs-ul mt-4">
              <li><strong class="text-slate-200">概览</strong>：账户余额、用量周期统计；支持导出 CSV/JSON。</li>
              <li><strong class="text-slate-200">可用模型</strong>：当前分组下可调用的模型 ID 列表，与 <code class="docs-code">GET /v1/models</code> 的数据来源一致。</li>
              <li><strong class="text-slate-200">API Key</strong>：创建、更新备注、配置 Key 级 IP 白名单、启用或禁用；删除后该 Key 立即失效。</li>
              <li><strong class="text-slate-200">调用日志</strong>：按时间、模型、HTTP 状态等筛选，便于对账与排错。</li>
              <li><strong class="text-slate-200">我的分组</strong>：查看所属分组名称、倍率等（只读）。</li>
              <li><strong class="text-slate-200">充值中心 / 兑换码</strong>：视部署是否开放线上充值；兑换码由管理员批量生成。</li>
              <li><strong class="text-slate-200">账户设置</strong>：修改密码、昵称、Webhook、账号级 IP 白名单等。</li>
            </ul>
            <p v-if="auth.role === 'admin' || auth.role === 'superadmin'" class="mt-4 rounded-lg border border-amber-500/20 bg-amber-950/25 px-4 py-3 text-sm text-amber-100/90">
              当前账号为管理员：还可进入 <RouterLink to="/admin" class="font-medium text-cyan-200 underline-offset-2 hover:underline">管理后台</RouterLink> 维护用户、渠道、分组、模型市场等。
            </p>
          </section>

          <section id="doc-auth" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>认证与安全</h2>
            <h3>网关 <code class="docs-code">/v1</code></h3>
            <pre class="docs-pre mt-2">Authorization: Bearer &lt;API Key&gt;</pre>
            <p class="mt-3">API Key 前缀由部署配置 <code class="docs-code">site.api_key_prefix</code> 决定（默认形如 <code class="docs-code">sk-th-</code>）。</p>
            <h3 class="mt-6">用户 JSON API <code class="docs-code">/api/user</code></h3>
            <p>除注册、登录、刷新外，均需在 Header 携带：</p>
            <pre class="docs-pre mt-2">Authorization: Bearer &lt;access_token&gt;</pre>
            <h3 class="mt-6">浏览器跨域（CORS）</h3>
            <p>服务端默认在 CORS 中允许的请求头包括：<code class="docs-code">Authorization</code>、<code class="docs-code">Content-Type</code>、<code class="docs-code">X-Request-ID</code>、<code class="docs-code">X-TraceHub-Session</code>、<code class="docs-code">X-Sticky-Session</code>。若您从浏览器直连网关且需额外自定义头，请与运维确认 <code class="docs-code">site.cors_origins</code> 与中间件配置。</p>
            <h3 class="mt-6">最佳实践</h3>
            <ul class="docs-ul">
              <li>勿把 API Key 写进前端公开仓库；由后端代理调用或放在私密环境变量中。</li>
              <li>为生产 Key 配置 IP 白名单与最小权限（禁用不用的 Key）。</li>
              <li>定期在控制台轮换密钥并查看异常调用日志。</li>
            </ul>
          </section>

          <section id="doc-gateway" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>网关 <code class="docs-code">/v1</code>（OpenAI 兼容）</h2>
            <p>以下路径均挂载在 <code class="docs-code">/v1</code> 下，请求/响应形态遵循 OpenAI 官方 API 文档，除非本页另有说明。实际是否可用取决于管理员是否为您的分组配置了对应上游能力。</p>

            <h3 class="mt-6">聊天与文本</h3>
            <ul class="docs-endpoint-list font-mono text-xs text-slate-300">
              <li><span class="text-emerald-400/90">POST</span> /v1/chat/completions — 聊天补全，支持 SSE</li>
              <li><span class="text-emerald-400/90">POST</span> /v1/completions — Legacy 文本补全</li>
            </ul>

            <h3 class="mt-6">向量与审核</h3>
            <ul class="docs-endpoint-list font-mono text-xs text-slate-300">
              <li><span class="text-emerald-400/90">POST</span> /v1/embeddings</li>
              <li><span class="text-emerald-400/90">POST</span> /v1/moderations</li>
            </ul>

            <h3 class="mt-6">图像</h3>
            <ul class="docs-endpoint-list font-mono text-xs text-slate-300">
              <li><span class="text-emerald-400/90">POST</span> /v1/images/generations</li>
              <li><span class="text-emerald-400/90">POST</span> /v1/images/edits</li>
              <li><span class="text-emerald-400/90">POST</span> /v1/images/variations</li>
            </ul>

            <h3 class="mt-6">音频</h3>
            <ul class="docs-endpoint-list font-mono text-xs text-slate-300">
              <li><span class="text-emerald-400/90">POST</span> /v1/audio/transcriptions</li>
              <li><span class="text-emerald-400/90">POST</span> /v1/audio/translations</li>
              <li><span class="text-emerald-400/90">POST</span> /v1/audio/speech</li>
            </ul>

            <h3 class="mt-6">模型列表</h3>
            <ul class="docs-endpoint-list font-mono text-xs text-slate-300">
              <li><span class="text-sky-400/90">GET</span> /v1/models</li>
              <li><span class="text-sky-400/90">GET</span> /v1/models/:model_id</li>
            </ul>

            <h3 class="mt-6">Responses API</h3>
            <ul class="docs-endpoint-list font-mono text-xs text-slate-300">
              <li><span class="text-emerald-400/90">POST</span> /v1/responses</li>
              <li><span class="text-sky-400/90">GET</span> /v1/responses/:response_id</li>
              <li><span class="text-rose-400/90">DELETE</span> /v1/responses/:response_id</li>
              <li><span class="text-emerald-400/90">POST</span> /v1/responses/:response_id/cancel</li>
            </ul>

            <h3 class="mt-6">Realtime（会话、通话、WebSocket 等）</h3>
            <p class="text-sm text-slate-500">包含 <code class="docs-code">/v1/realtime/sessions</code>、<code class="docs-code">/v1/realtime/calls</code>、<code class="docs-code">/v1/realtime</code> WebSocket 等一组端点；部分子资源依赖 Redis 映射或需在 Query / 头中显式指定 <code class="docs-code">model</code>。集成前请结合仓库内实现与上游文档。</p>

            <h3 class="mt-6">文件、批处理、微调</h3>
            <ul class="docs-endpoint-list font-mono text-xs text-slate-300">
              <li><span class="text-emerald-400/90">POST</span> /v1/files · <span class="text-sky-400/90">GET</span> /v1/files · <span class="text-sky-400/90">GET</span> /v1/files/:file_id · …</li>
              <li><span class="text-emerald-400/90">POST</span> /v1/batches · <span class="text-sky-400/90">GET</span> /v1/batches/:batch_id · …</li>
              <li><span class="text-emerald-400/90">POST</span> /v1/fine_tuning/jobs · <span class="text-sky-400/90">GET</span> /v1/fine_tuning/jobs · …</li>
            </ul>

            <h3 class="mt-6">Assistants / Threads / Runs / Vector stores</h3>
            <p class="text-sm text-slate-400">
              提供与 OpenAI Assistants API 形态一致的 CRUD 与运行接口（如 <code class="docs-code">/v1/assistants</code>、<code class="docs-code">/v1/threads/…</code>、<code class="docs-code">/v1/vector_stores/…</code> 等）。部分请求在 JSON 中不含 <code class="docs-code">model</code> 时，需通过 Query <code class="docs-code">?model=</code> 或请求头 <code class="docs-code">X-TraceHub-Model</code> 指定用于选路的模型 ID。
            </p>

            <h3 class="mt-6">可选：OpenAI 透传</h3>
            <p class="text-sm text-slate-400">
              若部署开启 <code class="docs-code">site.openai_passthrough</code>，未单独实现的其它 <code class="docs-code">/v1/…</code> 路径可能由透传处理器转发；允许/禁止的路径前缀以配置为准。生产环境务必收紧白名单。
            </p>
          </section>

          <section id="doc-headers" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>选路与会话头</h2>
            <table class="docs-table mt-4">
              <thead>
                <tr>
                  <th>头名称</th>
                  <th>说明</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td><code class="docs-code">X-TraceHub-Session</code></td>
                  <td>可选。粘性会话标识：相同值有助于将连续请求路由到同一逻辑会话/上游上下文（具体行为依渠道与模型）。</td>
                </tr>
                <tr>
                  <td><code class="docs-code">X-Sticky-Session</code></td>
                  <td>与上类似的历史别名，部分客户端或文档可能使用该名称。</td>
                </tr>
                <tr>
                  <td><code class="docs-code">X-TraceHub-Model</code></td>
                  <td>在请求体不含 <code class="docs-code">model</code> 字段时（如部分资源 GET、Assistants 等），用于显式指定选路模型；也可使用 Query <code class="docs-code">?model=</code>。</td>
                </tr>
              </tbody>
            </table>
            <p class="mt-4 text-sm text-slate-500">聊天补全 <code class="docs-code">POST /v1/chat/completions</code> 必须在 JSON 中提供 <code class="docs-code">model</code> 与合法的 <code class="docs-code">messages</code>。</p>
          </section>

          <section id="doc-user-api" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>用户 JSON API（<code class="docs-code">/api/user</code>）</h2>
            <p class="text-sm text-slate-400">供控制台与自建客户端调用；响应为 JSON，错误时返回相应 HTTP 状态码与消息体。</p>
            <table class="docs-table mt-4 text-left text-xs">
              <thead>
                <tr>
                  <th>方法</th>
                  <th>路径</th>
                  <th>说明</th>
                </tr>
              </thead>
              <tbody class="text-slate-400">
                <tr><td class="text-emerald-400/90">POST</td><td class="font-mono text-slate-300">/api/user/register</td><td>注册</td></tr>
                <tr><td class="text-emerald-400/90">POST</td><td class="font-mono text-slate-300">/api/user/login</td><td>登录，返回 token</td></tr>
                <tr><td class="text-emerald-400/90">POST</td><td class="font-mono text-slate-300">/api/user/refresh</td><td>刷新 access_token</td></tr>
                <tr><td class="text-sky-400/90">GET</td><td class="font-mono text-slate-300">/api/user/me</td><td>当前用户信息</td></tr>
                <tr><td class="text-sky-400/90">GET</td><td class="font-mono text-slate-300">/api/user/balance</td><td>余额</td></tr>
                <tr><td class="text-sky-400/90">GET</td><td class="font-mono text-slate-300">/api/user/usage-logs</td><td>调用日志分页</td></tr>
                <tr><td class="text-sky-400/90">GET</td><td class="font-mono text-slate-300">/api/user/group</td><td>我的分组</td></tr>
                <tr><td class="text-sky-400/90">GET</td><td class="font-mono text-slate-300">/api/user/available-models</td><td>可用模型（同 <code class="docs-code">/api/user/models</code>）</td></tr>
                <tr><td class="text-sky-400/90">GET</td><td class="font-mono text-slate-300">/api/user/usage</td><td>用量统计</td></tr>
                <tr><td class="text-sky-400/90">GET</td><td class="font-mono text-slate-300">/api/user/usage/export</td><td>用量导出</td></tr>
                <tr><td class="text-sky-400/90">GET</td><td class="font-mono text-slate-300">/api/user/apikeys</td><td>列出 API Key（不含完整密钥）</td></tr>
                <tr><td class="text-emerald-400/90">POST</td><td class="font-mono text-slate-300">/api/user/apikeys</td><td>创建 API Key</td></tr>
                <tr><td class="text-amber-400/90">PUT</td><td class="font-mono text-slate-300">/api/user/apikeys/:id</td><td>更新 Key 元数据</td></tr>
                <tr><td class="text-rose-400/90">DELETE</td><td class="font-mono text-slate-300">/api/user/apikeys/:id</td><td>删除 Key</td></tr>
                <tr><td class="text-emerald-400/90">POST</td><td class="font-mono text-slate-300">/api/user/redeem</td><td>兑换码充值</td></tr>
                <tr><td class="text-amber-400/90">PUT</td><td class="font-mono text-slate-300">/api/user/password</td><td>修改密码</td></tr>
                <tr><td class="text-amber-400/90">PUT</td><td class="font-mono text-slate-300">/api/user/bind</td><td>绑定资料（昵称等）</td></tr>
                <tr><td class="text-amber-400/90">PUT</td><td class="font-mono text-slate-300">/api/user/ip_whitelist</td><td>账号级 IP 白名单</td></tr>
              </tbody>
            </table>
          </section>

          <section id="doc-admin-api" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>管理端 API（<code class="docs-code">/api/admin</code>）</h2>
            <p>需 JWT，且用户角色为 <code class="docs-code">admin</code> 或 <code class="docs-code">superadmin</code>。涵盖仪表盘摘要、用户与余额调整、渠道与分组 CRUD、分组模型倍率、日志查询与导出、兑换码批次、系统配置读写与热加载、模型市场来源与条目维护等。</p>
            <p class="mt-3 text-sm text-slate-500">完整路径列表以 <code class="docs-code">internal/server/router.go</code> 中 <code class="docs-code">/api/admin</code> 注册为准；字段定义见 <code class="docs-code">docs/development-docs/api-design.md</code>。</p>
          </section>

          <section id="doc-ops" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>健康与观测</h2>
            <ul class="docs-endpoint-list font-mono text-xs text-slate-300">
              <li><span class="text-sky-400/90">GET</span> /health — 存活探针</li>
              <li><span class="text-sky-400/90">GET</span> /ready — 就绪（含数据库、Redis 等依赖检查）</li>
              <li><span class="text-sky-400/90">GET</span> /metrics — Prometheus 指标</li>
            </ul>
            <p class="mt-4 text-sm text-slate-500">以上路径<strong class="text-slate-400">不需要</strong> API Key，供编排与监控系统调用。</p>
          </section>

          <section id="doc-errors" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>错误与排查</h2>
            <p>网关错误体一般为 OpenAI 风格的 JSON（含 <code class="docs-code">error</code> 对象）。常见情况：</p>
            <ul class="docs-ul mt-3">
              <li><strong class="text-slate-200">401</strong>：API Key 缺失、错误、已禁用或不在白名单。</li>
              <li><strong class="text-slate-200">402</strong>：余额不足以完成本次预估扣费。</li>
              <li><strong class="text-slate-200">403</strong>：模型不在分组允许列表、未分配分组、选路失败等权限类原因。</li>
              <li><strong class="text-slate-200">400</strong>：请求体不合法（缺少 <code class="docs-code">model</code>、<code class="docs-code">messages</code> 无效等）。</li>
            </ul>
            <p class="mt-4 text-sm text-slate-500">若返回上游原始错误，请结合调用日志中的状态码与 <code class="docs-code">X-Request-ID</code> 追踪单次请求。</p>
          </section>

          <section id="doc-faq" class="docs-section th-glass rounded-[var(--th-radius-lg)] p-6 md:p-8">
            <h2>常见问题</h2>
            <dl class="docs-dl mt-4">
              <dt>为什么 <code class="docs-code">GET /v1/models</code> 是空的？</dt>
              <dd>通常表示当前分组未配置任何可用模型，或渠道/模型市场未同步。请联系管理员检查分组与渠道绑定。</dd>
              <dt>能否用 OpenAI 官方 SDK？</dt>
              <dd>可以。将 <code class="docs-code">baseURL</code> 指向您的 TraceHub 根地址（保留 <code class="docs-code">/v1</code> 路径约定），<code class="docs-code">apiKey</code> 填控制台发放的 Key 即可。</dd>
              <dt>流式输出中断？</dt>
              <dd>检查客户端超时、反向代理缓冲（需关闭对 SSE 的缓冲）、以及上游是否提前结束连接。</dd>
              <dt>文档与线上行为不一致？</dt>
              <dd>以您部署的版本与配置为准；升级后请关注发布说明或对比仓库路由代码。</dd>
            </dl>
          </section>

          <section class="th-glass--subtle rounded-[var(--th-radius-lg)] p-5 text-xs leading-relaxed text-slate-500">
            仓库延伸阅读：<code class="docs-code">README.zh-CN.md</code>、<code class="docs-code">docs/产品文档.md</code>、<code class="docs-code">docs/development-docs/deployment-ops.md</code>（部署与运维）。
          </section>
        </article>
      </main>
    </div>

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
  line-height: 1.55;
  color: rgb(203, 213, 225);
}
.docs-prose .docs-section h2 {
  margin: 0;
  font-size: 1.125rem;
  font-weight: 600;
  color: rgb(241 245 249);
  letter-spacing: -0.02em;
}
.docs-prose .docs-section h3 {
  margin: 0;
  margin-top: 1rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: rgb(203 213 225);
}
.docs-prose .docs-section p {
  margin-top: 0.75rem;
  font-size: 0.875rem;
  line-height: 1.65;
  color: rgb(148 163 184);
}
.docs-prose .docs-section p:first-of-type {
  margin-top: 1rem;
}
.docs-ol {
  list-style: decimal;
  padding-left: 1.25rem;
  font-size: 0.875rem;
  line-height: 1.65;
  color: rgb(148 163 184);
}
.docs-ol li {
  margin-top: 0.5rem;
}
.docs-ul {
  list-style: disc;
  padding-left: 1.25rem;
  font-size: 0.875rem;
  line-height: 1.65;
  color: rgb(148 163 184);
}
.docs-ul li {
  margin-top: 0.35rem;
}
.docs-dl dt {
  margin-top: 1rem;
  font-size: 0.8125rem;
  font-weight: 600;
  color: rgb(226 232 240);
}
.docs-dl dt:first-child {
  margin-top: 0;
}
.docs-dl dd {
  margin-top: 0.35rem;
  font-size: 0.8125rem;
  line-height: 1.6;
  color: rgb(148 163 184);
}
.docs-callout {
  font-size: 0.8125rem;
  line-height: 1.6;
  color: rgb(148 163 184);
}
.docs-endpoint-list {
  margin-top: 0.5rem;
  list-style: none;
  padding: 0;
}
.docs-endpoint-list li {
  margin-top: 0.35rem;
  padding: 0.35rem 0.5rem;
  border-radius: 0.375rem;
  background: rgba(0, 0, 0, 0.2);
}
.docs-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8125rem;
}
.docs-table th,
.docs-table td {
  border: 1px solid rgba(255, 255, 255, 0.08);
  padding: 0.5rem 0.65rem;
  vertical-align: top;
}
.docs-table th {
  background: rgba(255, 255, 255, 0.04);
  color: rgb(203 213 225);
  font-weight: 600;
  text-align: left;
}
.docs-toc-link {
  border-radius: 0.375rem;
  color: rgb(148 163 184);
  transition: color 0.15s, background 0.15s;
}
.docs-toc-link:hover {
  color: rgb(226 232 240);
  background: rgba(255, 255, 255, 0.04);
}
.docs-toc-link--active {
  color: rgb(165 243 254);
  background: rgba(6, 182, 212, 0.08);
}
.docs-mobile-toc-btn:hover {
  background: rgba(255, 255, 255, 0.06);
}
</style>
