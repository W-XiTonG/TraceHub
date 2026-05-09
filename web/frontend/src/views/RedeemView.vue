<script setup lang="ts">
import { ref } from 'vue'
import { http, type Envelope } from '../api/http'

const code = ref('')
const msg = ref('')
const result = ref<Record<string, unknown> | null>(null)

async function redeem() {
  msg.value = ''
  result.value = null
  try {
    const { data } = await http.post<Envelope<Record<string, unknown>>>('/api/user/redeem', { code: code.value })
    if (data.code !== 0) throw new Error(data.message || '兑换失败')
    msg.value = data.message || '成功'
    result.value = data.data
  } catch (e: unknown) {
    msg.value = e instanceof Error ? e.message : '兑换失败'
  }
}
</script>

<template>
  <div class="max-w-md">
    <h1 class="mb-6 text-2xl font-bold">兑换码</h1>
    <input v-model="code" placeholder="输入兑换码" class="mb-4 w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2" />
    <button type="button" class="rounded-lg bg-cyan-500 px-4 py-2 font-semibold text-slate-950 hover:bg-cyan-400" @click="redeem">兑换</button>
    <p class="mt-4 text-sm text-slate-400">{{ msg }}</p>
    <pre v-if="result" class="mt-4 overflow-auto rounded-lg bg-slate-900 p-4 text-xs">{{ JSON.stringify(result, null, 2) }}</pre>
  </div>
</template>
