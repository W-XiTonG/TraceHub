import { computed, ref, watch, type Ref } from 'vue'
import {
  formatUpstreamPrice,
  quotaPer1KFromUpstreamPrice,
  upstreamPriceFromQuotaPer1K,
  type SitePricingFactors,
} from '../utils/upstreamQuotaDerive'

type QuotaRow = { quota_in_per_1k: string; quota_out_per_1k: string }
type QuotaSide = 'in' | 'out'

export function useQuotaPer1kUpstreamField(opts: {
  codeRefShowQuotaOnly: Ref<boolean>
  deriveCurrency: Ref<'cny' | 'usd'>
  deriveDenom: Ref<number>
  deriveMargin: Ref<string>
  sitePricingFactors: () => SitePricingFactors
  parseDeriveMargin: () => number | null
}) {
  const focusedQuotaCell = ref<string | null>(null)
  const quotaEditBuf = ref(new Map<string, string>())

  watch(
    [opts.codeRefShowQuotaOnly, opts.deriveCurrency, opts.deriveDenom, opts.deriveMargin],
    () => {
      focusedQuotaCell.value = null
      quotaEditBuf.value = new Map()
    },
  )

  const priceUnitLabel = computed(() => {
    if (opts.codeRefShowQuotaOnly.value) return ''
    const cur = opts.deriveCurrency.value.toUpperCase()
    if (opts.deriveDenom.value === 1_000_000) return `${cur}/1M`
    if (opts.deriveDenom.value === 1000) return `${cur}/1K`
    return `${cur}/1`
  })

  function cellKey(idx: number, side: QuotaSide) {
    return `${idx}-${side}`
  }

  function getRaw(r: QuotaRow, side: QuotaSide) {
    return side === 'in' ? r.quota_in_per_1k : r.quota_out_per_1k
  }

  function setRaw(r: QuotaRow, side: QuotaSide, v: string) {
    if (side === 'in') r.quota_in_per_1k = v
    else r.quota_out_per_1k = v
  }

  function quotaToUpstreamDisplay(raw: string): string {
    const s = raw.trim()
    if (s === '') return ''
    const n = Number(s)
    if (!Number.isFinite(n) || !Number.isInteger(n) || n < 0) return s
    const margin = opts.parseDeriveMargin()
    if (margin === null) return s
    const p = upstreamPriceFromQuotaPer1K(
      opts.sitePricingFactors(),
      opts.deriveCurrency.value,
      opts.deriveDenom.value,
      n,
      margin,
    )
    if (p === null) return s
    return formatUpstreamPrice(p)
  }

  function quotaInputDisplay(r: QuotaRow, idx: number, side: QuotaSide): string {
    const key = cellKey(idx, side)
    if (focusedQuotaCell.value === key) {
      return quotaEditBuf.value.get(key) ?? ''
    }
    const raw = getRaw(r, side)
    if (opts.codeRefShowQuotaOnly.value) return raw
    return quotaToUpstreamDisplay(raw)
  }

  function onQuotaFocus(r: QuotaRow, idx: number, side: QuotaSide) {
    const key = cellKey(idx, side)
    focusedQuotaCell.value = key
    const raw = getRaw(r, side)
    const display = opts.codeRefShowQuotaOnly.value ? raw : quotaToUpstreamDisplay(raw) || raw
    quotaEditBuf.value.set(key, display)
  }

  function onQuotaInput(idx: number, side: QuotaSide, ev: Event) {
    quotaEditBuf.value.set(cellKey(idx, side), (ev.target as HTMLInputElement).value)
  }

  function commitQuotaBlur(r: QuotaRow, idx: number, side: QuotaSide) {
    const key = cellKey(idx, side)
    const text = quotaEditBuf.value.get(key) ?? getRaw(r, side)
    focusedQuotaCell.value = null
    quotaEditBuf.value.delete(key)

    const trimmed = text.trim()
    if (opts.codeRefShowQuotaOnly.value) {
      if (trimmed === '') {
        setRaw(r, side, '')
        return
      }
      const n = Number(trimmed)
      if (Number.isFinite(n) && Number.isInteger(n) && n >= 0) {
        setRaw(r, side, String(n))
      }
      return
    }

    if (trimmed === '') {
      setRaw(r, side, '')
      return
    }
    const price = Number(trimmed)
    if (!Number.isFinite(price) || price < 0) return
    const margin = opts.parseDeriveMargin()
    if (margin === null) return
    const q = quotaPer1KFromUpstreamPrice(
      opts.sitePricingFactors(),
      opts.deriveCurrency.value,
      opts.deriveDenom.value,
      price,
      margin,
    )
    if (q === null) return
    setRaw(r, side, String(q))
  }

  function flushQuotaEdits(rowList: QuotaRow[]) {
    const key = focusedQuotaCell.value
    if (!key) return
    const m = /^(\d+)-(in|out)$/.exec(key)
    if (!m) return
    const idx = Number(m[1])
    const side = m[2] as QuotaSide
    const r = rowList[idx]
    if (r) commitQuotaBlur(r, idx, side)
  }

  return {
    priceUnitLabel,
    quotaInputDisplay,
    onQuotaFocus,
    onQuotaInput,
    commitQuotaBlur,
    flushQuotaEdits,
  }
}
