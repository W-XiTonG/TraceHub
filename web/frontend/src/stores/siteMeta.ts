import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import axios from 'axios'
import { http, type Envelope } from '../api/http'

type PubSiteMeta = {
  brand_name?: string
  brand_icon_url?: string
}

export const useSiteMetaStore = defineStore('siteMeta', () => {
  const brandName = ref('')
  const brandIconUrl = ref('')
  const loaded = ref(false)
  let inflight: Promise<void> | null = null

  const displayName = computed(() => {
    const s = brandName.value.trim()
    return s || 'TraceHub'
  })

  const displayIconUrl = computed(() => {
    const u = brandIconUrl.value.trim()
    return u || '/tracehub-icon.svg'
  })

  async function ensureLoaded() {
    if (loaded.value) return
    if (inflight) return inflight
    inflight = (async () => {
      try {
        const { data } = await http.get<Envelope<PubSiteMeta>>('/api/pub/site-meta')
        if (data.code === 0 && data.data) {
          if (typeof data.data.brand_name === 'string') {
            brandName.value = data.data.brand_name
          }
          if (typeof data.data.brand_icon_url === 'string') {
            brandIconUrl.value = data.data.brand_icon_url
          }
        }
      } catch (e: unknown) {
        if (!axios.isAxiosError(e) || e.response?.status !== 404) {
          /* 保持默认 */
        }
      } finally {
        loaded.value = true
        inflight = null
      }
    })()
    return inflight
  }

  async function refreshFromServer() {
    loaded.value = false
    brandName.value = ''
    brandIconUrl.value = ''
    await ensureLoaded()
  }

  return {
    brandName,
    brandIconUrl,
    loaded,
    displayName,
    displayIconUrl,
    ensureLoaded,
    refreshFromServer,
  }
})
