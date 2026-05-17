import { onUnmounted, ref } from 'vue'

export type SmoothLoadingOptions = {
  /** 请求开始后延迟多久才显示加载 UI，避免快请求闪烁（ms） */
  showDelay?: number
  /** 加载 UI 一旦显示，至少保持多久（ms） */
  minVisible?: number
}

/**
 * 平滑加载态：延迟显示 + 最短展示，避免快请求时骨架屏/遮罩一闪而过。
 * - `busy`：请求进行中（用于禁用按钮等）
 * - `visible`：是否展示骨架屏/遮罩/转圈
 */
export function useSmoothLoading(opts: SmoothLoadingOptions = {}) {
  const showDelay = opts.showDelay ?? 300
  const minVisible = opts.minVisible ?? 320

  const busy = ref(false)
  const visible = ref(false)

  let depth = 0
  let showTimer: ReturnType<typeof setTimeout> | null = null
  let hideTimer: ReturnType<typeof setTimeout> | null = null
  let shownAt = 0

  function clearShowTimer() {
    if (showTimer !== null) {
      clearTimeout(showTimer)
      showTimer = null
    }
  }

  function clearHideTimer() {
    if (hideTimer !== null) {
      clearTimeout(hideTimer)
      hideTimer = null
    }
  }

  function start() {
    depth++
    if (depth > 1) return
    busy.value = true
    clearHideTimer()
    clearShowTimer()
    showTimer = setTimeout(() => {
      showTimer = null
      if (depth === 0 || !busy.value) return
      visible.value = true
      shownAt = Date.now()
    }, showDelay)
  }

  function endVisible(): Promise<void> {
    return new Promise((resolve) => {
      clearShowTimer()
      if (!visible.value) {
        resolve()
        return
      }
      const remain = minVisible - (Date.now() - shownAt)
      if (remain <= 0) {
        visible.value = false
        resolve()
        return
      }
      hideTimer = setTimeout(() => {
        hideTimer = null
        if (depth === 0) visible.value = false
        resolve()
      }, remain)
    })
  }

  async function end() {
    depth = Math.max(0, depth - 1)
    if (depth > 0) return
    busy.value = false
    await endVisible()
  }

  async function run<T>(fn: () => Promise<T>): Promise<T> {
    start()
    try {
      return await fn()
    } finally {
      await end()
    }
  }

  onUnmounted(() => {
    depth = 0
    busy.value = false
    clearShowTimer()
    clearHideTimer()
    visible.value = false
  })

  return { busy, visible, start, end, run }
}
