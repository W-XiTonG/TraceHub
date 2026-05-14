/**
 * 将 `<input type="datetime-local">` 的值（本地无时区）转为 RFC3339（UTC），供后端 `time.RFC3339` 解析。
 */
export function localDateTimeToRFC3339(localValue: string): string | undefined {
  const s = localValue.trim()
  if (!s) return undefined
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return undefined
  return d.toISOString()
}

type DateTimeInput = HTMLInputElement & { showPicker?: () => Promise<void> }

/**
 * 在用户点击整块控件时弹出原生日期时间面板（避免只能点右侧小日历图标）。
 * Chromium 要求控件已聚焦且处于同一次用户手势内调用 `showPicker()`，否则会异步 reject；
 * 从覆盖层 button 点进来时若未先 focus，常见现象是「点了没反应」。
 * 面板正常关闭（Promise resolve）后再 blur，减轻分段高亮残留；reject 则保持聚焦便于点日历图标。
 */
export function openDateTimePickerField(el: HTMLInputElement | null): void {
  if (!el) return

  const scheduleBlur = () => {
    window.setTimeout(() => {
      try {
        el.blur()
      } catch {
        /* ignore */
      }
    }, 0)
  }

  try {
    el.focus({ preventScroll: true })
  } catch {
    try {
      el.focus()
    } catch {
      return
    }
  }

  const typed = el as DateTimeInput
  if (typeof typed.showPicker !== 'function') return

  try {
    const ret = typed.showPicker()
    if (ret != null && typeof (ret as Promise<void>).then === 'function') {
      void (ret as Promise<void>).then(scheduleBlur, () => {
        /* SecurityError / NotSupportedError 等：输入框已聚焦 */
      })
    } else {
      scheduleBlur()
    }
  } catch {
    /* 同步抛错时输入框已聚焦 */
  }
}
