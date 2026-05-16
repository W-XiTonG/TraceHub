package gateway

import "testing"

func TestParseVideoBillableSeconds_UsageFirst(t *testing.T) {
	body := []byte(`{"status":"completed","seconds":"8","usage":{"seconds":12.5}}`)
	sec := parseVideoBillableSeconds(body)
	if sec != 13 {
		t.Fatalf("want 13 from usage.seconds, got %d", sec)
	}
}

func TestParseVideoBillableSeconds_FallbackTopLevel(t *testing.T) {
	body := []byte(`{"status":"completed","seconds":"6"}`)
	sec := parseVideoBillableSeconds(body)
	if sec != 6 {
		t.Fatalf("got %d", sec)
	}
}

func TestParseVideoBillableSeconds_Duration(t *testing.T) {
	body := []byte(`{"status":"completed","duration":10}`)
	sec := parseVideoBillableSeconds(body)
	if sec != 10 {
		t.Fatalf("got %d", sec)
	}
}
