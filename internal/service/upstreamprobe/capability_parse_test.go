package upstreamprobe

import "testing"

func TestParseUpstreamErrorBody_GetGoAPI(t *testing.T) {
	body := []byte(`{"code":"invalid_api_platform","message":"invalid api platform: 48","data":null}`)
	code, summary := ParseUpstreamErrorBody(body)
	if code != "invalid_api_platform" {
		t.Fatalf("code=%q", code)
	}
	if summary != "invalid api platform: 48" {
		t.Fatalf("summary=%q", summary)
	}
}

func TestParseUpstreamErrorBody_Truncate(t *testing.T) {
	long := make([]byte, 300)
	for i := range long {
		long[i] = 'x'
	}
	_, summary := ParseUpstreamErrorBody(long)
	if len(summary) > maxErrorSummaryLen {
		t.Fatalf("len=%d", len(summary))
	}
}
