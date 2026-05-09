<script setup lang="ts">
import { RouterLink } from 'vue-router'
</script>

<template>
  <div class="max-w-2xl">
    <h1 class="mb-2 text-2xl font-bold">充值中心</h1>
    <p class="mb-6 text-sm text-slate-500">产品文档中的微信 / 支付宝 / Stripe 自助充值将在此接入；当前可通过兑换码或联系平台管理员调整余额。</p>
    <div class="space-y-4 rounded-xl border border-slate-800 bg-slate-900/60 p-6">
      <div class="flex items-start gap-3">
        <span class="mt-0.5 text-cyan-400">①</span>
        <div>
          <p class="font-medium text-slate-200">兑换码</p>
          <p class="mt-1 text-sm text-slate-400">输入兑换码即时增加额度。</p>
          <RouterLink to="/console/redeem" class="mt-2 inline-block text-sm text-cyan-400 hover:underline">前往兑换码 →</RouterLink>
        </div>
      </div>
      <div class="flex items-start gap-3">
        <span class="mt-0.5 text-cyan-400">②</span>
        <div>
          <p class="font-medium text-slate-200">对公 / 人工充值</p>
          <p class="mt-1 text-sm text-slate-400">企业付款、发票与套餐请联系管理员在后台为您调账。</p>
        </div>
      </div>
      <div class="flex items-start gap-3 border-t border-slate-800 pt-4">
        <span class="mt-0.5 text-amber-500">!</span>
        <p class="text-sm text-slate-500">上线支付后，将在此展示套餐价目表与支付结果回调说明。</p>
      </div>
    </div>
  </div>
</template>
