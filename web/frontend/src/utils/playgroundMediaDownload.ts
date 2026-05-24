import { http } from '../api/http'

const MIME_EXT: Record<string, string> = {
  'image/png': 'png',
  'image/jpeg': 'jpg',
  'image/jpg': 'jpg',
  'image/webp': 'webp',
  'image/gif': 'gif',
  'video/mp4': 'mp4',
  'video/webm': 'webm',
}

function extFromDataUrl(src: string): string | null {
  const m = /^data:([^;,]+)/.exec(src.trim())
  if (!m) return null
  return MIME_EXT[m[1]] ?? null
}

function extFromHttpUrl(src: string): string | null {
  try {
    const path = new URL(src).pathname
    const dot = path.lastIndexOf('.')
    if (dot >= 0) {
      const ext = path.slice(dot + 1).toLowerCase()
      if (/^[a-z0-9]{1,5}$/.test(ext)) return ext
    }
  } catch {
    /* ignore */
  }
  return null
}

function defaultFilename(kind: 'image' | 'video', ext: string, base?: string): string {
  const stem = base?.replace(/[^\w.-]+/g, '_').slice(0, 48) || `tracehub-playground-${kind}`
  const stamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)
  return `${stem}-${stamp}.${ext}`
}

export function triggerBlobDownload(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.rel = 'noopener'
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

function saveBlob(blob: Blob, kind: 'image' | 'video', filenameBase: string | undefined, srcHint: string): boolean {
  if (!blob.size) return false
  const fallbackExt = kind === 'video' ? 'mp4' : 'png'
  const ext = MIME_EXT[blob.type] ?? extFromDataUrl(srcHint) ?? extFromHttpUrl(srcHint) ?? fallbackExt
  triggerBlobDownload(blob, defaultFilename(kind, ext, filenameBase))
  return true
}

async function downloadViaUserProxy(
  url: string,
  kind: 'image' | 'video',
  filenameBase?: string,
): Promise<boolean> {
  if (!localStorage.getItem('tracehub_token')) return false
  try {
    const res = await http.get<Blob>('/api/user/playground/media', {
      params: { url },
      responseType: 'blob',
      timeout: 120000,
    })
    return saveBlob(res.data, kind, filenameBase, url)
  } catch {
    return false
  }
}

function openResourceInNewTab(url: string): void {
  const a = document.createElement('a')
  a.href = url
  a.target = '_blank'
  a.rel = 'noopener noreferrer'
  document.body.appendChild(a)
  a.click()
  a.remove()
}

/** 将试玩区生成的图片或视频保存到本地。 */
export async function downloadPlaygroundMedia(
  src: string,
  kind: 'image' | 'video',
  filenameBase?: string,
): Promise<'ok' | 'opened' | 'fail'> {
  const t = src.trim()
  if (!t) return 'fail'

  try {
    if (t.startsWith('data:') || t.startsWith('blob:')) {
      const res = await fetch(t)
      if (!res.ok) return 'fail'
      const blob = await res.blob()
      return saveBlob(blob, kind, filenameBase, t) ? 'ok' : 'fail'
    }

    if (t.startsWith('http://') || t.startsWith('https://')) {
      try {
        const res = await fetch(t, { mode: 'cors', referrerPolicy: 'no-referrer' })
        if (res.ok) {
          const blob = await res.blob()
          if (saveBlob(blob, kind, filenameBase, t)) return 'ok'
        }
      } catch {
        /* CORS 或网络错误，改走服务端代理 */
      }

      if (await downloadViaUserProxy(t, kind, filenameBase)) return 'ok'

      openResourceInNewTab(t)
      return 'opened'
    }
  } catch {
    return 'fail'
  }
  return 'fail'
}
