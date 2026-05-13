import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import axios from 'axios'
import { http, type Envelope } from '../api/http'

export function apiErrorMessage(e: unknown, fallback: string) {
  if (axios.isAxiosError(e)) {
    const d = e.response?.data
    if (d && typeof d === 'object' && 'message' in d) {
      const m = (d as { message?: unknown }).message
      if (typeof m === 'string' && m.trim()) return m.trim()
    }
  }
  if (e instanceof Error) return e.message
  return fallback
}

const LS_USER = 'tracehub_user'

export type AuthUserBrief = {
  id: number
  email: string
  username?: string
  balance?: number
  group_name?: string
}

export type RegisterEmailCodeMeta = {
  required: boolean
  can_send: boolean
  /** 服务端 Redis 客户端非 nil（与启动时 Ping 结果一致） */
  redis_ok?: boolean
  /** 已配置 smtp_host 与 from */
  smtp_ready?: boolean
}

/** GET /api/user/login-captcha */
export type LoginCaptchaPayload = { captcha_id: string; captcha_answer: string }

export type LoginCaptchaResponse =
  | { enabled: false }
  | { enabled: true; captcha_id: string; image_base64: string }

function readStoredUser(): AuthUserBrief | null {
  try {
    const s = localStorage.getItem(LS_USER)
    if (!s) return null
    return JSON.parse(s) as AuthUserBrief
  } catch {
    return null
  }
}

/** JWT payload（access），与后端 jwtx.Claims 字段一致 */
type AccessPayload = {
  uid?: number
  email?: string
  role?: string
}

function parseAccessPayload(access: string): AccessPayload | null {
  try {
    const parts = access.split('.')
    if (parts.length < 2) return null
    return JSON.parse(atob(parts[1])) as AccessPayload
  } catch {
    return null
  }
}

/** 仅有 token、无本地 user 时，用 JWT 里的 uid/email 补全展示（不写敏感信息） */
function tryHydrateUserFromToken(t: string, current: AuthUserBrief | null): AuthUserBrief | null {
  if (current && (current.email?.trim() || current.username?.trim())) return current
  const p = parseAccessPayload(t)
  if (!p) return current
  const id = typeof p.uid === 'number' && !Number.isNaN(p.uid) ? p.uid : 0
  const email = (p.email && String(p.email).trim()) || ''
  if (!email && !id) return current
  const brief: AuthUserBrief = { id, email }
  if (email) {
    localStorage.setItem(LS_USER, JSON.stringify(brief))
  }
  return brief
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('tracehub_token') || '')
  const user = ref<AuthUserBrief | null>(readStoredUser())
  const role = ref('')

  const isLoggedIn = computed(() => !!token.value)

  /** 顶栏展示：优先用户名 → 邮箱 → JWT 邮箱 → 用户 #id */
  const accountDisplay = computed(() => {
    const u = user.value
    if (u?.username?.trim()) return u.username.trim()
    if (u?.email?.trim()) return u.email.trim()
    const p = token.value ? parseAccessPayload(token.value) : null
    if (p?.email?.trim()) return p.email.trim()
    if (p?.uid != null && Number(p.uid) > 0) return `用户 #${p.uid}`
    return ''
  })

  /** 头像内单字：取展示串中第一个字母、数字或汉字；「用户 #id」用 U，避免无意义首字 */
  const avatarInitial = computed(() => {
    const s = accountDisplay.value.trim()
    if (!s) return '?'
    if (/^用户 #\d+$/.test(s)) return 'U'
    const m = s.match(/[A-Za-z0-9\u4e00-\u9fff]/)
    if (m) {
      const c = m[0]
      return /[a-z]/.test(c) ? c.toUpperCase() : c
    }
    return '?'
  })

  function setRoleFromToken(access: string) {
    const p = parseAccessPayload(access)
    if (p?.role) role.value = p.role
  }

  if (token.value) {
    setRoleFromToken(token.value)
    user.value = tryHydrateUserFromToken(token.value, user.value)
  }

  function setSession(access: string, u: typeof user.value, r?: string) {
    token.value = access
    user.value = u
    if (!user.value) {
      user.value = tryHydrateUserFromToken(access, null)
    }
    if (r) role.value = r
    localStorage.setItem('tracehub_token', access)
    if (user.value) {
      localStorage.setItem(LS_USER, JSON.stringify(user.value))
    } else {
      localStorage.removeItem(LS_USER)
    }
    setRoleFromToken(access)
  }

  function logout() {
    token.value = ''
    user.value = null
    role.value = ''
    localStorage.removeItem('tracehub_token')
    localStorage.removeItem(LS_USER)
  }

  /** account：邮箱或用户名（请求体仍使用字段名 email 以兼容后端）；captcha 在服务端开启登录验证码时必填 */
  async function login(account: string, password: string, captcha?: LoginCaptchaPayload) {
    try {
      const body: Record<string, string> = {
        email: account.trim(),
        password,
      }
      if (captcha) {
        body.captcha_id = captcha.captcha_id
        body.captcha_answer = captcha.captcha_answer
      }
      const { data } = await http.post<
        Envelope<{ access_token: string; user: typeof user.value }>
      >('/api/user/login', body)
      if (data.code !== 0) throw new Error(data.message || '登录失败')
      setSession(data.data.access_token, data.data.user)
    } catch (e) {
      throw new Error(apiErrorMessage(e, '登录失败'))
    }
  }

  async function fetchLoginCaptcha(): Promise<LoginCaptchaResponse> {
    try {
      const { data } = await http.get<Envelope<LoginCaptchaResponse>>('/api/user/login-captcha')
      if (data.code !== 0) throw new Error(data.message || '加载失败')
      return data.data ?? { enabled: false }
    } catch (e) {
      throw new Error(apiErrorMessage(e, '加载失败'))
    }
  }

  /** 发送登录邮箱验证码（需后端启用 email 且 Redis 可用）；captcha 在开启登录图形验证码时必填 */
  async function sendLoginCode(
    email: string,
    locale?: string,
    captcha?: LoginCaptchaPayload,
  ): Promise<number | undefined> {
    try {
      const body: Record<string, string> = { email: email.trim() }
      if (locale) body.locale = locale
      if (captcha) {
        body.captcha_id = captcha.captcha_id
        body.captcha_answer = captcha.captcha_answer
      }
      const { data } = await http.post<Envelope<{ next_resend_sec?: number }>>(
        '/api/user/login-code/send',
        body,
      )
      if (data.code !== 0) throw new Error(data.message || '发送失败')
      return data.data?.next_resend_sec
    } catch (e) {
      // 保留 AxiosError，便于登录页按 HTTP 状态区分「图形验证码」与业务错误
      if (axios.isAxiosError(e)) throw e
      throw e instanceof Error ? e : new Error(apiErrorMessage(e, '发送失败'))
    }
  }

  async function loginWithEmailCode(email: string, code: string) {
    try {
      const { data } = await http.post<
        Envelope<{ access_token: string; user: typeof user.value }>
      >('/api/user/login-code/verify', { email: email.trim(), code: code.trim() })
      if (data.code !== 0) throw new Error(data.message || '登录失败')
      setSession(data.data.access_token, data.data.user)
    } catch (e) {
      throw new Error(apiErrorMessage(e, '登录失败'))
    }
  }

  async function fetchRegisterEmailCodeMeta(): Promise<RegisterEmailCodeMeta> {
    try {
      const { data } = await http.get<Envelope<RegisterEmailCodeMeta>>(
        '/api/user/register/email-code-meta',
      )
      if (data.code !== 0) throw new Error(data.message || '加载失败')
      return data.data ?? { required: false, can_send: false }
    } catch (e) {
      throw new Error(apiErrorMessage(e, '加载失败'))
    }
  }

  async function sendRegisterCode(email: string, locale?: string): Promise<number | undefined> {
    try {
      const { data } = await http.post<Envelope<{ next_resend_sec?: number }>>(
        '/api/user/register/email-code/send',
        { email: email.trim(), ...(locale ? { locale } : {}) },
      )
      if (data.code !== 0) throw new Error(data.message || '发送失败')
      return data.data?.next_resend_sec
    } catch (e) {
      throw new Error(apiErrorMessage(e, '发送失败'))
    }
  }

  async function register(email: string, password: string, username?: string, code?: string) {
    try {
      const body: Record<string, string | undefined> = {
        email,
        password,
        username,
      }
      if (code !== undefined) body.code = code
      const { data } = await http.post<Envelope<null>>('/api/user/register', body)
      if (data.code !== 0) throw new Error(data.message || '注册失败')
    } catch (e) {
      throw new Error(apiErrorMessage(e, '注册失败'))
    }
  }

  return {
    token,
    user,
    role,
    isLoggedIn,
    accountDisplay,
    avatarInitial,
    login,
    fetchLoginCaptcha,
    sendLoginCode,
    loginWithEmailCode,
    fetchRegisterEmailCodeMeta,
    sendRegisterCode,
    register,
    logout,
    setSession,
  }
})
