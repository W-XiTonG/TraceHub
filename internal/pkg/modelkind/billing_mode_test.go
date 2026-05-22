package modelkind

import "testing"

func TestUsageBillingMode_videoSeconds(t *testing.T) {
	if got := UsageBillingMode("doubao-seedance-1-5-pro-251215", 8, 0); got != UsageBillingVideoDuration {
		t.Fatalf("got %q want video_duration", got)
	}
}

func TestUsageBillingMode_chatTokens(t *testing.T) {
	if got := UsageBillingMode("gpt-4o-mini", 100, 50); got != UsageBillingToken {
		t.Fatalf("got %q want token", got)
	}
}
