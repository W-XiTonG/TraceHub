import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import axios from 'axios'
import { http, type Envelope } from '../api/http'

type PubSiteMeta = {
  brand_name?: string
  brand_icon_url?: string
}

/** 与后端 sitecfg.DefaultBrandIconURL 一致；接口返回该值时存为空串表示「无自定义」 */
const defaultPubIcon = '/tracehub-icon.svg'

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
    return u || defaultPubIcon
  })

  async function pullPubSiteMeta(): Promise<void> {
    try {
      const { data } = await http.get<Envelope<PubSiteMeta>>('/api/pub/site-meta', {
        params: { _: Date.now() },
      })
      if (Number(data.code) === 0 && data.data) {
        if (typeof data.data.brand_name === 'string') {
          brandName.value = data.data.brand_name
        }
        if (typeof data.data.brand_icon_url === 'string') {
          const u = data.data.brand_icon_url.trim()
          brandIconUrl.value = u === defaultPubIcon ? '' : u
        }
      }
    } catch (e: unknown) {
      if (!axios.isAxiosError(e) || e.response?.status !== 404) {
        /* 保持默认 */
      }
    }
  }

  async function ensureLoaded() {
    if (loaded.value) return
    if (inflight) return inflight
    inflight = (async () => {
      try {
        await pullPubSiteMeta()
      } finally {
        loaded.value = true
        inflight = null
      }
    })()
    return inflight
  }

  async function refreshFromServer() {
    if (inflight) {
      await inflight.catch(() => {})
    }
    loaded.value = false
    brandName.value = ''
    brandIconUrl.value = ''
    inflight = (async () => {
      try {
        await pullPubSiteMeta()
      } finally {
        loaded.value = true
        inflight = null
      }
    })()
    await inflight
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
