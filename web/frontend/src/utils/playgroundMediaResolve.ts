/** 尝试将远程图片 URL 转为 blob object URL（CORS 允许时）。 */
export async function resolveImageUrlForDisplay(src: string): Promise<string | null> {
  const t = src.trim()
  if (!t) return null
  if (t.startsWith('data:image/') || t.startsWith('blob:')) return t
  if (!t.startsWith('http://') && !t.startsWith('https://')) return null
  try {
    const res = await fetch(t, { mode: 'cors', referrerPolicy: 'no-referrer' })
    if (!res.ok) return null
    const blob = await res.blob()
    if (!blob.type.startsWith('image/')) return null
    return URL.createObjectURL(blob)
  } catch {
    return null
  }
}

export function isResolvableHttpImage(src: string): boolean {
  const t = src.trim()
  return t.startsWith('http://') || t.startsWith('https://')
}
