/** MD5 hex（Gravatar / Cravatar 邮箱哈希） */
function md5Hex(input: string): string {
  const utf8 = new TextEncoder().encode(input)
  const words = md5Words(utf8)
  let hex = ''
  for (let i = 0; i < 4; i++) {
    for (let j = 0; j < 4; j++) {
      const b = (words[i] >>> ((j * 8) & 31)) & 0xff
      hex += b.toString(16).padStart(2, '0')
    }
  }
  return hex
}

function md5Words(bytes: Uint8Array): number[] {
  const n = bytes.length
  const withLen = new Uint8Array(((n + 8 >>> 6) + 1) * 64)
  withLen.set(bytes)
  withLen[n] = 0x80
  const bitLen = n * 8
  withLen[withLen.length - 8] = bitLen & 0xff
  withLen[withLen.length - 7] = (bitLen >>> 8) & 0xff
  withLen[withLen.length - 6] = (bitLen >>> 16) & 0xff
  withLen[withLen.length - 5] = (bitLen >>> 24) & 0xff

  let a = 0x67452301
  let b = 0xefcdab89
  let c = 0x98badcfe
  let d = 0x10325476

  const rot = (x: number, s: number) => (x << s) | (x >>> (32 - s))
  const ff = (aa: number, bb: number, cc: number, dd: number, x: number, s: number, t: number) => {
    aa = (aa + ((bb & cc) | (~bb & dd)) + x + t) | 0
    return (bb + rot(aa, s)) | 0
  }
  const gg = (aa: number, bb: number, cc: number, dd: number, x: number, s: number, t: number) => {
    aa = (aa + ((bb & dd) | (cc & ~dd)) + x + t) | 0
    return (bb + rot(aa, s)) | 0
  }
  const hh = (aa: number, bb: number, cc: number, dd: number, x: number, s: number, t: number) => {
    aa = (aa + (bb ^ cc ^ dd) + x + t) | 0
    return (bb + rot(aa, s)) | 0
  }
  const ii = (aa: number, bb: number, cc: number, dd: number, x: number, s: number, t: number) => {
    aa = (aa + (cc ^ (bb | ~dd)) + x + t) | 0
    return (bb + rot(aa, s)) | 0
  }

  const k = new Int32Array(64)
  for (let i = 0; i < 64; i++) k[i] = Math.floor(Math.abs(Math.sin(i + 1)) * 0x100000000)

  for (let i = 0; i < withLen.length; i += 64) {
    const x = new Int32Array(16)
    for (let j = 0; j < 16; j++) {
      const o = i + j * 4
      x[j] = withLen[o] | (withLen[o + 1] << 8) | (withLen[o + 2] << 16) | (withLen[o + 3] << 24)
    }
    let aa = a
    let bb = b
    let cc = c
    let dd = d
    aa = ff(aa, bb, cc, dd, x[0], 7, k[0]); dd = ff(dd, aa, bb, cc, x[1], 12, k[1])
    cc = ff(cc, dd, aa, bb, x[2], 17, k[2]); bb = ff(bb, cc, dd, aa, x[3], 22, k[3])
    aa = ff(aa, bb, cc, dd, x[4], 7, k[4]); dd = ff(dd, aa, bb, cc, x[5], 12, k[5])
    cc = ff(cc, dd, aa, bb, x[6], 17, k[6]); bb = ff(bb, cc, dd, aa, x[7], 22, k[7])
    aa = ff(aa, bb, cc, dd, x[8], 7, k[8]); dd = ff(dd, aa, bb, cc, x[9], 12, k[9])
    cc = ff(cc, dd, aa, bb, x[10], 17, k[10]); bb = ff(bb, cc, dd, aa, x[11], 22, k[11])
    aa = ff(aa, bb, cc, dd, x[12], 7, k[12]); dd = ff(dd, aa, bb, cc, x[13], 12, k[13])
    cc = ff(cc, dd, aa, bb, x[14], 17, k[14]); bb = ff(bb, cc, dd, aa, x[15], 22, k[15])
    aa = gg(aa, bb, cc, dd, x[1], 5, k[16]); dd = gg(dd, aa, bb, cc, x[6], 9, k[17])
    cc = gg(cc, dd, aa, bb, x[11], 14, k[18]); bb = gg(bb, cc, dd, aa, x[0], 20, k[19])
    aa = gg(aa, bb, cc, dd, x[5], 5, k[20]); dd = gg(dd, aa, bb, cc, x[10], 9, k[21])
    cc = gg(cc, dd, aa, bb, x[15], 14, k[22]); bb = gg(bb, cc, dd, aa, x[4], 20, k[23])
    aa = gg(aa, bb, cc, dd, x[9], 5, k[24]); dd = gg(dd, aa, bb, cc, x[14], 9, k[25])
    cc = gg(cc, dd, aa, bb, x[3], 14, k[26]); bb = gg(bb, cc, dd, aa, x[8], 20, k[27])
    aa = gg(aa, bb, cc, dd, x[13], 5, k[28]); dd = gg(dd, aa, bb, cc, x[2], 9, k[29])
    cc = gg(cc, dd, aa, bb, x[7], 14, k[30]); bb = gg(bb, cc, dd, aa, x[12], 20, k[31])
    aa = hh(aa, bb, cc, dd, x[5], 4, k[32]); dd = hh(dd, aa, bb, cc, x[8], 11, k[33])
    cc = hh(cc, dd, aa, bb, x[11], 16, k[34]); bb = hh(bb, cc, dd, aa, x[14], 23, k[35])
    aa = hh(aa, bb, cc, dd, x[1], 4, k[36]); dd = hh(dd, aa, bb, cc, x[4], 11, k[37])
    cc = hh(cc, dd, aa, bb, x[7], 16, k[38]); bb = hh(bb, cc, dd, aa, x[10], 23, k[39])
    aa = hh(aa, bb, cc, dd, x[13], 4, k[40]); dd = hh(dd, aa, bb, cc, x[0], 11, k[41])
    cc = hh(cc, dd, aa, bb, x[3], 16, k[42]); bb = hh(bb, cc, dd, aa, x[6], 23, k[43])
    aa = hh(aa, bb, cc, dd, x[9], 4, k[44]); dd = hh(dd, aa, bb, cc, x[12], 11, k[45])
    cc = hh(cc, dd, aa, bb, x[15], 16, k[46]); bb = hh(bb, cc, dd, aa, x[2], 23, k[47])
    aa = ii(aa, bb, cc, dd, x[0], 6, k[48]); dd = ii(dd, aa, bb, cc, x[7], 10, k[49])
    cc = ii(cc, dd, aa, bb, x[14], 15, k[50]); bb = ii(bb, cc, dd, aa, x[5], 21, k[51])
    aa = ii(aa, bb, cc, dd, x[12], 6, k[52]); dd = ii(dd, aa, bb, cc, x[3], 10, k[53])
    cc = ii(cc, dd, aa, bb, x[10], 15, k[54]); bb = ii(bb, cc, dd, aa, x[1], 21, k[55])
    aa = ii(aa, bb, cc, dd, x[8], 6, k[56]); dd = ii(dd, aa, bb, cc, x[15], 10, k[57])
    cc = ii(cc, dd, aa, bb, x[6], 15, k[58]); bb = ii(bb, cc, dd, aa, x[13], 21, k[59])
    aa = ii(aa, bb, cc, dd, x[4], 6, k[60]); dd = ii(dd, aa, bb, cc, x[11], 10, k[61])
    cc = ii(cc, dd, aa, bb, x[2], 15, k[62]); bb = ii(bb, cc, dd, aa, x[9], 21, k[63])
    a = (a + aa) | 0
    b = (b + bb) | 0
    c = (c + cc) | 0
    d = (d + dd) | 0
  }
  return [a, b, c, d]
}

/** 根据邮箱生成头像 URL（Gravatar 协议；优先国内可访问的 Cravatar 镜像） */
export function gravatarUrlFromEmail(email: string, size = 80): string {
  const normalized = email.trim().toLowerCase()
  if (!normalized || !normalized.includes('@')) return ''
  const hash = md5Hex(normalized)
  const q = `s=${size}&d=404&r=g`
  return `https://cravatar.cn/avatar/${hash}?${q}`
}
