<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useAuthStore } from '../../stores/auth'
import { gravatarUrlFromEmail } from '../../utils/gravatarUrl'

const auth = useAuthStore()
const imgFailed = ref(false)

const avatarUrl = computed(() => {
  const email = auth.userEmail
  if (!email) return ''
  return gravatarUrlFromEmail(email, 64)
})

const showPhoto = computed(() => !!avatarUrl.value && !imgFailed.value)

watch(
  () => auth.userEmail,
  () => {
    imgFailed.value = false
  },
)

function onImgError() {
  imgFailed.value = true
}
</script>

<template>
  <div
    class="th-pg-v2-turn__avatar th-pg-v2-turn__avatar--user"
    :class="{ 'th-pg-v2-turn__avatar--photo': showPhoto }"
  >
    <img
      v-if="showPhoto"
      :src="avatarUrl"
      alt=""
      class="th-pg-v2-turn__avatar-img"
      @error="onImgError"
    />
    <span v-else>{{ auth.avatarInitial }}</span>
  </div>
</template>
