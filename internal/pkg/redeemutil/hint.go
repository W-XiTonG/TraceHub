package redeemutil

import "strings"

const maxCodeHintLen = 80 // 与 ent schema redeem_ledgers.code_hint 一致

// CodeHint 与后台列表一致：长码中间脱敏，便于展示与对账时人工核对。
func CodeHint(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	var out string
	if len(code) <= 10 {
		out = code
	} else {
		out = code[:6] + "****" + code[len(code)-4:]
	}
	if len(out) > maxCodeHintLen {
		return out[:maxCodeHintLen]
	}
	return out
}
