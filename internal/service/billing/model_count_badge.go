package billing

import "strconv"

// ModelCountHundredPlusBadge 将模型条数格式化为官网展示用文案：百位向下取整并加「+」；
// 低于 300 返回空串（不展示）。
func ModelCountHundredPlusBadge(n int) string {
	if n < 300 {
		return ""
	}
	return strconv.Itoa((n/100)*100) + "+"
}
