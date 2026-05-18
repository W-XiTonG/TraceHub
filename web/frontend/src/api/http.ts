import axios from 'axios'

const browserOrigin =
  typeof window !== 'undefined' && window.location?.origin ? window.location.origin : ''

export const http = axios.create({
  // 使用绝对 origin，避免部分 HTTP 代理将相对路径解析成无效 URL
  baseURL: browserOrigin,
  timeout: 120000,
})

/** 登录/注册等接口返回 401 属正常业务，勿触发全局登出 */
const PUBLIC_USER_AUTH_PATHS = [
  '/api/user/login',
  '/api/user/register',
  '/api/user/login-captcha',
  '/api/user/login-code/send',
  '/api/user/login-code/verify',
  '/api/user/register/email-code/send',
  '/api/user/register/email-code-meta',
  '/api/user/refresh',
]

export function isPublicUserAuthApi(url: string | undefined): boolean {
  if (!url) return false
  const path = url.split('?')[0]
  return PUBLIC_USER_AUTH_PATHS.some((p) => path === p || path.endsWith(p))
}

let handlingSessionExpiry = false
let onSessionExpired: (() => void) | undefined

/** 在 main.ts 挂载 router 后注册，避免拦截器里动态 import 路由导致异常跳转 */
export function registerSessionExpiredHandler(fn: () => void) {
  onSessionExpired = fn
}

function redirectToLoginAfterSessionExpiry() {
  if (handlingSessionExpiry || !onSessionExpired) return
  handlingSessionExpiry = true
  try {
    onSessionExpired()
  } finally {
    window.setTimeout(() => {
      handlingSessionExpiry = false
    }, 800)
  }
}

http.interceptors.request.use((config) => {
  const token = localStorage.getItem('tracehub_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

http.interceptors.response.use(
  (res) => res,
  (error) => {
    if (axios.isAxiosError(error) && error.response?.status === 401) {
      const url = error.config?.url
      if (!isPublicUserAuthApi(url)) {
        redirectToLoginAfterSessionExpiry()
      }
    }
    return Promise.reject(error)
  },
)

export type Envelope<T> = { code: number; message?: string; data: T }
