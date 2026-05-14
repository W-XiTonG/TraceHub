import { getOrCreateClientScopeId } from './clientScopeId'

/** 与推广文案绑定：后台修改文案后指纹变化，角标按新文案重新计算 */
const LS_DISMISS_PREFIX = 'tracehub_register_promo_dismiss:v2:'
/** 旧版仅 sessionStorage，迁移到「本机 scope + 文案指纹」 */
const LEGACY_SESSION_FP = 'tracehub_register_promo_session_fp'

export function fingerprintForPromo(text: string): string {
  const t = text.trim()
  if (!t) return ''
  let h = 2166136261 >>> 0
  for (let i = 0; i < t.length; i++) {
    h ^= t.charCodeAt(i)
    h = Math.imul(h, 16777619) >>> 0
  }
  return h.toString(16)
}

function dismissStorageKey(scopeId: string, contentFp: string): string {
  return `${LS_DISMISS_PREFIX}${scopeId}:${contentFp}`
}

/** 当前浏览器作用域下，是否已对「该条」推广点过我知道了 */
export function isPromoDismissedForScope(contentFp: string): boolean {
  if (!contentFp || typeof localStorage === 'undefined') return false
  const scopeId = getOrCreateClientScopeId()
  try {
    if (localStorage.getItem(dismissStorageKey(scopeId, contentFp)) === '1') return true
    if (typeof sessionStorage !== 'undefined') {
      const leg = sessionStorage.getItem(LEGACY_SESSION_FP)
      if (leg === contentFp) {
        localStorage.setItem(dismissStorageKey(scopeId, contentFp), '1')
        sessionStorage.removeItem(LEGACY_SESSION_FP)
        return true
      }
    }
    return false
  } catch {
    return false
  }
}

export function dismissPromoForScope(contentFp: string): void {
  if (!contentFp || typeof localStorage === 'undefined') return
  const scopeId = getOrCreateClientScopeId()
  try {
    localStorage.setItem(dismissStorageKey(scopeId, contentFp), '1')
    if (typeof sessionStorage !== 'undefined') {
      sessionStorage.removeItem(LEGACY_SESSION_FP)
    }
  } catch {
    /* ignore quota / private mode */
  }
}
