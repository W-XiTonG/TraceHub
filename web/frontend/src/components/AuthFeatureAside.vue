<script setup lang="ts">
import { RouterLink } from 'vue-router'

defineProps<{
  /** 窄屏下压缩为更少条目与更小字号 */
  compact?: boolean
}>()

const items = [
  { t: 'OpenAI 兼容', d: '同一套 /v1 接口聚合多上游' },
  { t: '用量与导出', d: '日志可查，CSV / JSON 导出' },
  { t: '分组与计费', d: '倍率、余额、兑换码一体化' },
]
</script>

<template>
  <aside
    class="text-slate-400"
    :class="compact ? 'max-w-md' : 'max-w-md lg:max-w-sm xl:max-w-md'"
    aria-label="TraceHub 产品要点"
  >
    <RouterLink
      to="/"
      class="group mb-6 inline-flex items-center gap-2 rounded-lg text-sm font-medium text-slate-500 transition hover:text-cyan-400/90"
    >
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-4 w-4 -translate-x-0.5 transition group-hover:-translate-x-1" aria-hidden="true">
        <path fill-rule="evenodd" d="M17 10a.75.75 0 01-.75.75H5.612l4.158 3.96a.75.75 0 11-1.04 1.08l-5.5-5.25a.75.75 0 010-1.08l5.5-5.25a.75.75 0 111.04 1.08L5.612 9.25H16.25A.75.75 0 0117 10z" clip-rule="evenodd" />
      </svg>
      返回官网
    </RouterLink>
    <p class="th-kicker mb-2"><span class="th-kicker-accent">TraceHub</span></p>
    <h2 class="text-xl font-semibold tracking-tight text-slate-100" :class="compact ? 'text-lg' : 'md:text-2xl'">企业级 AI API 网关</h2>
    <p class="mt-2 text-sm leading-relaxed text-slate-500" :class="compact && 'text-xs'">
      控制台涵盖密钥、用量、充值与分组；管理员可配置渠道与模型市场。
    </p>
    <ul class="mt-8 space-y-4" :class="compact && 'mt-5 space-y-3'">
      <li
        v-for="(it, i) in items"
        :key="it.t"
        v-show="!compact || i < 2"
        class="flex gap-3"
      >
        <span class="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-cyan-500/75" aria-hidden="true" />
        <div>
          <p class="text-sm font-medium text-slate-300">{{ it.t }}</p>
          <p class="mt-0.5 text-xs leading-relaxed text-slate-600">{{ it.d }}</p>
        </div>
      </li>
    </ul>
    <RouterLink
      to="/#features"
      class="mt-8 inline-flex items-center gap-1 text-sm font-medium text-cyan-400/90 transition hover:text-cyan-300"
      :class="compact && 'mt-5 text-xs'"
    >
      查看官网功能详解
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-4 w-4" aria-hidden="true">
        <path fill-rule="evenodd" d="M3 10a.75.75 0 01.75-.75h10.638L10.23 5.29a.75.75 0 111.04-1.08l5.5 5.25a.75.75 0 010 1.08l-5.5 5.25a.75.75 0 11-1.04-1.08l4.158-3.96H3.75A.75.75 0 013 10z" clip-rule="evenodd" />
      </svg>
    </RouterLink>
  </aside>
</template>
