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

func TestIsVideoTerminalFailureStatus(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{"failed", true},
		{"error", true},
		{"cancelled", true},
		{"canceled", true},
		{"FAILED", true},
		{"completed", false},
		{"in_progress", false},
		{"queued", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isVideoTerminalFailureStatus(tc.status); got != tc.want {
			t.Fatalf("status %q: got %v want %v", tc.status, got, tc.want)
		}
	}
}

func TestParseVideoStatusFromJSON_failed(t *testing.T) {
	st, _ := parseVideoStatusFromJSON([]byte(`{"status":"failed","error":{"message":"blocked"}}`))
	if st != "failed" {
		t.Fatalf("got %q", st)
	}
	if !isVideoTerminalFailureStatus(st) {
		t.Fatal("failed status should be terminal failure")
	}
}
