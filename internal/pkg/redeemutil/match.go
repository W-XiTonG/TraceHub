package redeemutil

import (
	"strings"

	"tracehub/internal/ent/predicate"
	"tracehub/internal/ent/redeemcode"
)

// CodeMatchPredicate 与用户核销接口一致：去首尾空白后，原样与大写双分支匹配（兼容大小写混存的历史数据）。
func CodeMatchPredicate(trimmed string) predicate.RedeemCode {
	trimmed = strings.TrimSpace(trimmed)
	upper := strings.ToUpper(trimmed)
	if trimmed == upper {
		return redeemcode.CodeEQ(trimmed)
	}
	return redeemcode.Or(redeemcode.CodeEQ(trimmed), redeemcode.CodeEQ(upper))
}

// CodePrefixPredicate 管理端前缀筛选：原样与大写双分支（与核销时大小写习惯一致）。
func CodePrefixPredicate(trimmed string) predicate.RedeemCode {
	trimmed = strings.TrimSpace(trimmed)
	upper := strings.ToUpper(trimmed)
	if trimmed == upper {
		return redeemcode.CodeHasPrefix(trimmed)
	}
	return redeemcode.Or(redeemcode.CodeHasPrefix(trimmed), redeemcode.CodeHasPrefix(upper))
}
