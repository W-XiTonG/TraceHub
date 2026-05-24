package gateway

// UsageLogMeta 控制 usage_logs 的 failover 相关字段。
type UsageLogMeta struct {
	UpstreamAttempt   int  // >=1
	IsClientOutcome   bool // true=面向客户端的最终结果（计费/统计）
	IsFailoverAttempt bool // true=failover 中间上游尝试（非客户端最终结果）
}

func defaultUsageLogMeta() UsageLogMeta {
	return UsageLogMeta{UpstreamAttempt: 1, IsClientOutcome: true, IsFailoverAttempt: false}
}

func metaForFailoverAttempt(attempt int) UsageLogMeta {
	if attempt < 1 {
		attempt = 1
	}
	return UsageLogMeta{UpstreamAttempt: attempt, IsClientOutcome: false, IsFailoverAttempt: true}
}

func metaForClientOutcome(attempt int) UsageLogMeta {
	if attempt < 1 {
		attempt = 1
	}
	return UsageLogMeta{UpstreamAttempt: attempt, IsClientOutcome: true, IsFailoverAttempt: false}
}
