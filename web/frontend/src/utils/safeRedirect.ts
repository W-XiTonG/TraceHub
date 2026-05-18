/** 仅允许站内相对路径，防止开放重定向与畸形 URL 被代理误解析 */
export function sanitizeAppRedirect(raw: string | undefined | null): string {
  const s = (raw ?? '').trim()
  if (!s.startsWith('/') || s.startsWith('//')) return ''
  if (s.includes('://') || s.includes('\\')) return ''
  return s
}
