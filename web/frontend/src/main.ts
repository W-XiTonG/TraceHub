import './theme-boot'
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { i18n } from './i18n'
import { registerSessionExpiredHandler } from './api/http'
import { useAuthStore } from './stores/auth'
import { sanitizeAppRedirect } from './utils/safeRedirect'
import './style.css'
import './themes.css'
import './themes-light.css'

const app = createApp(App)
app.config.errorHandler = (err, _instance, info) => {
  console.error('[TraceHub Vue]', info, err)
}
const pinia = createPinia()
app.use(pinia)
app.use(i18n)
app.use(router)

registerSessionExpiredHandler(() => {
  const auth = useAuthStore()
  const from = router.currentRoute.value.fullPath
  auth.logout()
  if (router.currentRoute.value.name === 'login') return
  const query: Record<string, string> = { reason: 'session_expired' }
  const back = sanitizeAppRedirect(from)
  if (back && back !== '/login' && !back.startsWith('/login?')) {
    query.redirect = back
  }
  void router.replace({ name: 'login', query })
})

app.mount('#app')
