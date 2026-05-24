package gatewayalert

import (
	"testing"

	"tracehub/internal/pkg/sitecfg"
)

func TestClassify(t *testing.T) {
	allOn := sitecfg.GatewayAdminAlertTriggers{
		Notify5xx: true, NotifyTransport: true, NotifyFailover: true, Notify4xx: true, Notify402: true,
	}
	cases := []struct {
		name string
		evt  Event
		tr   sitecfg.GatewayAdminAlertTriggers
		want string
		ok   bool
	}{
		{"failover", Event{IsFailoverAttempt: true, Status: 503}, allOn, "failover", true},
		{"402", Event{Status: 402}, allOn, "402", true},
		{"5xx", Event{Status: 502}, allOn, "5xx", true},
		{"transport zero", Event{Status: 0}, allOn, "transport", true},
		{"4xx", Event{Status: 429}, allOn, "4xx", true},
		{"ok 200", Event{Status: 200}, allOn, "", false},
		{"402 off", Event{Status: 402}, sitecfg.GatewayAdminAlertTriggers{Notify402: false}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kind, ok := Classify(tc.evt, tc.tr)
			if ok != tc.ok || kind != tc.want {
				t.Fatalf("Classify() = %q, %v; want %q, %v", kind, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestFingerprintStable(t *testing.T) {
	evt := Event{Model: "gpt-4", ChannelID: 3, Status: 502}
	a := Fingerprint("5xx", evt)
	b := Fingerprint("5xx", evt)
	if a != b {
		t.Fatalf("fingerprints differ: %q vs %q", a, b)
	}
}
