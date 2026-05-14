/**
 * Copy text to the clipboard. On non-secure HTTP (e.g. LAN IP), browsers often omit
 * or break the async Clipboard API; this falls back to execCommand which still
 * works from a direct user gesture.
 */
export async function copyTextToClipboard(text: string): Promise<void> {
  const value = String(text ?? '')
  if (!value) return

  const nav = typeof navigator !== 'undefined' ? navigator : undefined
  const clip = nav?.clipboard

  if (typeof window !== 'undefined' && window.isSecureContext && typeof clip?.writeText === 'function') {
    try {
      await clip.writeText(value)
      return
    } catch {
      /* fall through */
    }
  }

  copyViaExecCommand(value)
}

function copyViaExecCommand(text: string): void {
  const ta = document.createElement('textarea')
  ta.value = text
  ta.setAttribute('readonly', '')
  ta.setAttribute('aria-hidden', 'true')
  ta.style.position = 'fixed'
  ta.style.left = '-9999px'
  ta.style.top = '0'
  ta.style.opacity = '0'
  document.body.appendChild(ta)
  ta.focus()
  ta.select()
  let ok = false
  try {
    ok = document.execCommand('copy')
  } finally {
    ta.remove()
  }
  if (!ok) throw new Error('copy failed')
}
