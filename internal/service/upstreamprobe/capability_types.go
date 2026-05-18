package upstreamprobe

import (
	"strings"
	"time"
)

const (
	CapabilityVideo  = "video"
	CapabilityImage  = "image"
	CapabilityAudio  = "audio"

	CapStatusOK      = "ok"
	CapStatusFail    = "fail"
	CapStatusUnknown = "unknown"
	CapStatusSkipped = "skipped"

	CapSourceRuntime = "runtime"
	CapSourceProbe   = "probe"

	overlayKeyPrefix = "cap:"
)

// CapabilitySnapshot 运行时/探测写入的能力状态（覆盖层与 DB capabilities 子对象同形）。
type CapabilitySnapshot struct {
	Status       string    `json:"status"`
	CheckedAt    time.Time `json:"checked_at"`
	HTTPStatus   int       `json:"http_status,omitempty"`
	ErrorCode    string    `json:"error_code,omitempty"`
	ErrorSummary string    `json:"error_summary,omitempty"`
	Source       string    `json:"source,omitempty"`
}

func overlayRedisKey(channelID int, logicalModel, capability string) string {
	return overlayKeyPrefix + strings.TrimSpace(logicalModel) + ":" + capability + ":" + itoa(channelID)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func capRowFromMap(m map[string]interface{}) (CapabilitySnapshot, bool) {
	if m == nil {
		return CapabilitySnapshot{}, false
	}
	st, _ := m["status"].(string)
	st = strings.TrimSpace(st)
	if st == "" {
		return CapabilitySnapshot{}, false
	}
	out := CapabilitySnapshot{Status: st, Source: strings.TrimSpace(stringField(m, "source"))}
	if s := strings.TrimSpace(stringField(m, "checked_at")); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			out.CheckedAt = t
		}
	}
	switch v := m["http_status"].(type) {
	case float64:
		out.HTTPStatus = int(v)
	case int:
		out.HTTPStatus = v
	}
	out.ErrorCode = strings.TrimSpace(stringField(m, "error_code"))
	out.ErrorSummary = strings.TrimSpace(stringField(m, "error_summary"))
	return out, true
}

func stringField(m map[string]interface{}, k string) string {
	v, _ := m[k].(string)
	return v
}

func capRowToMap(s CapabilitySnapshot) map[string]interface{} {
	m := map[string]interface{}{
		"status":     s.Status,
		"checked_at": s.CheckedAt.UTC().Format(time.RFC3339),
		"source":     s.Source,
	}
	if s.HTTPStatus > 0 {
		m["http_status"] = s.HTTPStatus
	}
	if s.ErrorCode != "" {
		m["error_code"] = s.ErrorCode
	}
	if s.ErrorSummary != "" {
		m["error_summary"] = s.ErrorSummary
	}
	return m
}

// EffectiveCapStatus 将 fail 按 fail_stale_hours 降为 stale 语义（读路径）。
func EffectiveCapStatus(s CapabilitySnapshot, failStaleHours int, now time.Time) string {
	if s.Status != CapStatusFail {
		return s.Status
	}
	if failStaleHours <= 0 || s.CheckedAt.IsZero() {
		return CapStatusFail
	}
	if now.Sub(s.CheckedAt) >= time.Duration(failStaleHours)*time.Hour {
		return "stale"
	}
	return CapStatusFail
}
