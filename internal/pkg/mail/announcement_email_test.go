package mail

import (
	"strings"
	"testing"
)

func TestAnnouncementEmail(t *testing.T) {
	t.Parallel()
	subject, body := AnnouncementEmail("维护通知", "今晚 22:00 起停机 1 小时。", "https://example.com/console")
	if !strings.Contains(subject, "维护通知") {
		t.Fatalf("subject = %q", subject)
	}
	if !strings.Contains(body, "维护通知") || !strings.Contains(body, "今晚 22:00") {
		t.Fatalf("body missing content: %q", body)
	}
	if !strings.Contains(body, "https://example.com/console") {
		t.Fatalf("body missing console url: %q", body)
	}
	_, bodyNoURL := AnnouncementEmail("T", "C", "")
	if strings.Contains(bodyNoURL, "登录控制台") {
		t.Fatalf("empty console url should omit link line")
	}
}
