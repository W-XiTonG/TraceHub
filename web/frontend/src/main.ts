import './theme-boot'
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { i18n } from './i18n'
import './style.css'
import './themes.css'
import './themes-light.css'

const app = createApp(App)
app.config.errorHandler = (err, _instance, info) => {
  console.error('[TraceHub Vue]', info, err)
}
app.use(createPinia())
app.use(i18n)
app.use(router)
app.mount('#app')
