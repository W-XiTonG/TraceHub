/** 是否为可直接作为 <img src> 的图片地址 */
export function isLikelyImageUrl(url: string): boolean {
  const u = url.trim()
  if (!u) return false
  if (/^data:image\//i.test(u)) return true
  if (/\.(png|jpe?g|gif|webp|svg|bmp)(\?|#|$)/i.test(u)) return true
  return /[?&]format=(png|jpe?g)/i.test(u) || /\/qr\//i.test(u) || /qrcode=/i.test(u)
}

/** 易支付等返回的收银台链接（非图片）→ 生成可扫码的二维码图 */
export function checkoutQrImageSrc(checkoutUrl: string): string {
  const u = checkoutUrl.trim()
  if (!u) return ''
  if (isLikelyImageUrl(u)) return u
  if (!/^https?:\/\//i.test(u)) return ''
  return `https://api.qrserver.com/v1/create-qr-code/?size=240x240&margin=10&data=${encodeURIComponent(u)}`
}

export function paymentCheckoutUrl(order: { pay_url?: string; qr_code?: string } | null): string {
  if (!order) return ''
  return (order.pay_url || order.qr_code || '').trim()
}
