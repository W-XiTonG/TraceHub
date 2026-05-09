import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { http, type Envelope } from '../api/http'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('tracehub_token') || '')
  const user = ref<{ id: number; email: string; username?: string; balance?: number; group_name?: string } | null>(null)
  const role = ref('')

  const isLoggedIn = computed(() => !!token.value)

  function setSession(access: string, u: typeof user.value, r?: string) {
    token.value = access
    user.value = u
    if (r) role.value = r
    localStorage.setItem('tracehub_token', access)
    try {
      const payload = JSON.parse(atob(access.split('.')[1]))
      if (payload.role) role.value = payload.role
    } catch {
      /* ignore */
    }
  }

  function logout() {
    token.value = ''
    user.value = null
    role.value = ''
    localStorage.removeItem('tracehub_token')
  }

  async function login(email: string, password: string) {
    const { data } = await http.post<
      Envelope<{ access_token: string; user: typeof user.value }>
    >('/api/user/login', { email, password })
    if (data.code !== 0) throw new Error(data.message || '登录失败')
    setSession(data.data.access_token, data.data.user)
  }

  async function register(email: string, password: string, username?: string) {
    const { data } = await http.post<Envelope<null>>('/api/user/register', {
      email,
      password,
      username,
    })
    if (data.code !== 0) throw new Error(data.message || '注册失败')
  }

  return { token, user, role, isLoggedIn, login, register, logout, setSession }
})
