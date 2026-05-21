package klingvideo

import (
	"encoding/json"
	"testing"
)

func TestExtractModelField_model_name(t *testing.T) {
	body := []byte(`{"model_name":"kling-v3","prompt":"hi"}`)
	if got := ExtractModelField(body); got != "kling-v3" {
		t.Fatalf("got %q", got)
	}
}

func TestPatchModelInBody(t *testing.T) {
	out, err := PatchModelInBody([]byte(`{"model":"a"}`), "kling-v2.6-pro")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if m["model_name"] != "kling-v2.6-pro" || m["model"] != "kling-v2.6-pro" {
		t.Fatalf("%v", m)
	}
}
