<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import FeedbackModal from '../components/FeedbackModal.vue'

const router = useRouter()
const open = ref(false)

onMounted(() => {
  open.value = true
})

function onOpenChange(v: boolean) {
  open.value = v
  if (!v) {
    void router.replace({ name: 'console' })
  }
}
</script>

<template>
  <div class="min-h-[20vh]" aria-hidden="true">
    <FeedbackModal :model-value="open" @update:model-value="onOpenChange" />
  </div>
</template>
