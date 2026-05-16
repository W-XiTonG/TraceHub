package gateway

import (
	"testing"
)

func TestParseVideoCreateBody(t *testing.T) {
	body := []byte(`{"model":"sora-2","prompt":"cat","seconds":"8","size":"1280x720"}`)
	p, err := parseVideoCreateBody(body)
	if err != nil {
		t.Fatal(err)
	}
	if p.Model != "sora-2" || p.Seconds != "8" {
		t.Fatalf("%+v", p)
	}
	params := videoJobParamsFromParsed(p)
	if params.Seconds != 8 {
		t.Fatalf("seconds %d", params.Seconds)
	}
}

func TestParseVideoBillableSeconds(t *testing.T) {
	sec := parseVideoBillableSeconds([]byte(`{"usage":{"seconds":4.2}}`))
	if sec != 4 {
		t.Fatalf("got %d", sec)
	}
}

func TestParseVideoStatusFromJSON(t *testing.T) {
	st, sec := parseVideoStatusFromJSON([]byte(`{"status":"completed","seconds":"8"}`))
	if st != "completed" || sec != 8 {
		t.Fatalf("st=%s sec=%d", st, sec)
	}
}
