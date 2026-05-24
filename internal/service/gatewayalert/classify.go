package gatewayalert

import (
	"strings"

	"tracehub/internal/pkg/sitecfg"
)

// Event 描述一次可告警的网关 usage 写入。
type Event struct {
	RequestID         string
	UserID            int
	APIKeyID          int
	ChannelID         int
	Model             string
	Upstream          string
	Status            int
	ErrMsg            string
	IsFailoverAttempt bool
	IsClientOutcome   bool
}

// Classify 根据已开 trigger 判定是否告警及 event kind。
func Classify(evt Event, triggers sitecfg.GatewayAdminAlertTriggers) (kind string, ok bool) {
	if evt.IsFailoverAttempt && triggers.NotifyFailover {
		return "failover", true
	}
	if evt.Status == 402 && triggers.Notify402 {
		return "402", true
	}
	if evt.Status >= 500 && triggers.Notify5xx {
		return "5xx", true
	}
	if isTransport(evt) && triggers.NotifyTransport {
		return "transport", true
	}
	if evt.Status >= 400 && evt.Status < 500 && evt.Status != 402 && triggers.Notify4xx {
		return "4xx", true
	}
	return "", false
}

func isTransport(evt Event) bool {
	if evt.Status == 0 {
		return true
	}
	if evt.Status == 502 && strings.TrimSpace(evt.ErrMsg) != "" {
		return true
	}
	msg := strings.ToLower(strings.TrimSpace(evt.ErrMsg))
	if msg == "" {
		return false
	}
	for _, kw := range []string{"timeout", "dial", "connection refused", "connection reset", "tls", "eof", "transport", "i/o", "network"} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}

// Fingerprint 用于限流去重。
func Fingerprint(kind string, evt Event) string {
	model := strings.TrimSpace(evt.Model)
	if model == "" {
		model = "unknown"
	}
	statusClass := statusClassFor(evt.Status, kind)
	return kind + ":" + statusClass + ":" + model + ":" + itoa(evt.ChannelID)
}

func statusClassFor(status int, kind string) string {
	if kind == "failover" {
		return "failover"
	}
	if status == 0 {
		return "transport"
	}
	if status >= 500 {
		return "5xx"
	}
	if status >= 400 {
		return "4xx"
	}
	return "other"
}

func itoa(n int) string {
	if n <= 0 {
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
