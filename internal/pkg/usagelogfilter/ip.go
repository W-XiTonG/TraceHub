package usagelogfilter

import (
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/usagelog"
)

// ApplyIPAddress 按客户端 IP 片段筛选（大小写不敏感包含匹配；空串不追加条件）。
func ApplyIPAddress(q *ent.UsageLogQuery, raw string) *ent.UsageLogQuery {
	s := strings.TrimSpace(raw)
	if s == "" {
		return q
	}
	return q.Where(usagelog.IPAddressContainsFold(s))
}
