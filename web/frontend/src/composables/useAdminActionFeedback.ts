import axios from 'axios'
import { nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { apiErrorMessage } from '../stores/auth'

const FLASH_SCROLL_ID = 'admin-action-flash'

export function adminActionFlashScrollIntoView() {
  void nextTick(() => {
    document.getElementById(FLASH_SCROLL_ID)?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
  })
}

/** 管理端写操作统一成功/失败提示与 loading（举一反三复用）。 */
export function useAdminActionFeedback() {
  const { t } = useI18n()
  const flashMsg = ref('')
  const flashIsError = ref(false)
  const busy = ref(false)

  function clearFlash() {
    flashMsg.value = ''
    flashIsError.value = false
  }

  function flashSuccess(text: string) {
    flashIsError.value = false
    flashMsg.value = text
    adminActionFlashScrollIntoView()
  }

  function flashError(e: unknown, fallback?: string) {
    flashIsError.value = true
    flashMsg.value = apiErrorMessage(e, fallback ?? t('adminCommon.requestFail'))
    adminActionFlashScrollIntoView()
  }

  function flashInfo(text: string) {
    flashIsError.value = false
    flashMsg.value = text
    adminActionFlashScrollIntoView()
  }

  /**
   * 执行异步操作：可选校验、loading、成功/失败提示。
   * @returns 是否成功
   */
  async function runAction(opts: {
    action: () => Promise<void>
    successMessage?: string
    onSuccess?: () => void | Promise<void>
    validate?: () => string | null
  }): Promise<boolean> {
    clearFlash()
    const validationErr = opts.validate?.()
    if (validationErr) {
      flashIsError.value = true
      flashMsg.value = validationErr
      adminActionFlashScrollIntoView()
      return false
    }
    busy.value = true
    try {
      await opts.action()
      if (opts.successMessage) flashSuccess(opts.successMessage)
      await opts.onSuccess?.()
      return true
    } catch (e: unknown) {
      flashError(e)
      return false
    } finally {
      busy.value = false
    }
  }

  return {
    flashMsg,
    flashIsError,
    busy,
    clearFlash,
    flashSuccess,
    flashError,
    flashInfo,
    runAction,
    flashScrollId: FLASH_SCROLL_ID,
  }
}

/** 与渠道页等共用的 HTTP 错误文案（无 composable 时也可 import）。 */
export function adminHttpErrMessage(e: unknown, fallback: string): string {
  if (axios.isAxiosError(e) && e.response?.data && typeof e.response.data === 'object') {
    const d = e.response.data as { message?: string }
    if (typeof d.message === 'string' && d.message.trim()) {
      return d.message.trim()
    }
  }
  return e instanceof Error ? e.message : fallback
}
