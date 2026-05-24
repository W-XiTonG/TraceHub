import { computed, onMounted, ref, type CSSProperties } from 'vue'

const LS_KEY = 'trace-hub:pg-v2-debug-body-h'
const DEFAULT_PX = 60
const MIN_PX = 36

function readStoredHeight(): number {
  try {
    const raw = localStorage.getItem(LS_KEY)
    if (!raw) return DEFAULT_PX
    const n = Number.parseInt(raw, 10)
    return Number.isFinite(n) ? n : DEFAULT_PX
  } catch {
    return DEFAULT_PX
  }
}

function maxBodyPx(): number {
  if (typeof window === 'undefined') return 280
  return Math.min(Math.round(window.innerHeight * 0.35), 280)
}

function clampHeight(px: number): number {
  return Math.min(maxBodyPx(), Math.max(MIN_PX, Math.round(px)))
}

export function usePlaygroundDebugResize() {
  const bodyHeightPx = ref(DEFAULT_PX)
  const resizing = ref(false)

  const debugBodyStyle = computed(
    (): CSSProperties => ({
      '--th-pg-debug-body-h': `${bodyHeightPx.value}px`,
    }),
  )

  function persistHeight() {
    try {
      localStorage.setItem(LS_KEY, String(bodyHeightPx.value))
    } catch {
      /* ignore quota / private mode */
    }
  }

  function onResizePointerDown(e: PointerEvent) {
    const handle = e.currentTarget as HTMLElement | null
    if (!handle) return

    resizing.value = true
    handle.setPointerCapture(e.pointerId)

    const startY = e.clientY
    const startH = bodyHeightPx.value

    const onMove = (ev: PointerEvent) => {
      bodyHeightPx.value = clampHeight(startH + (startY - ev.clientY))
    }

    const finish = (ev: PointerEvent) => {
      resizing.value = false
      if (handle.hasPointerCapture(ev.pointerId)) {
        handle.releasePointerCapture(ev.pointerId)
      }
      handle.removeEventListener('pointermove', onMove)
      handle.removeEventListener('pointerup', finish)
      handle.removeEventListener('pointercancel', finish)
      persistHeight()
    }

    handle.addEventListener('pointermove', onMove)
    handle.addEventListener('pointerup', finish)
    handle.addEventListener('pointercancel', finish)
  }

  onMounted(() => {
    bodyHeightPx.value = clampHeight(readStoredHeight())
  })

  return {
    bodyHeightPx,
    resizing,
    debugBodyStyle,
    onResizePointerDown,
  }
}
