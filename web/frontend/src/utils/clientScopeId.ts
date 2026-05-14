/**
 * 本机浏览器侧稳定标识（首次访问生成 UUID 并写入 localStorage）。
 * 用于「我知道了」等纯前端偏好：按设备/浏览器配置隔离，不与其他用户或主机共享。
 * 非硬件级强指纹；清除站点数据后会重新生成。
 */
const LS_KEY = 'tracehub_client_scope_id'

function randomId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 14)}`
}

export function getOrCreateClientScopeId(): string {
  if (typeof localStorage === 'undefined') return 'no-ls'
  try {
    let id = localStorage.getItem(LS_KEY)?.trim()
    if (!id) {
      id = randomId()
      localStorage.setItem(LS_KEY, id)
    }
    return id
  } catch {
    return 'no-ls'
  }
}
