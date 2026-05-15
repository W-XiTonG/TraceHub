package gateway

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAugmentUpstreamJSONModelNotExistChatHint(t *testing.T) {
	in := []byte(`{"code":20012,"message":"Model does not exist. Please check it carefully.","data":null}`)
	out := augmentUpstreamJSONModelNotExistChatHint(in)
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	msg, _ := m["message"].(string)
	if !strings.Contains(msg, "images/generations") {
		t.Fatalf("expected hint in message, got %q", msg)
	}
}

func TestAugmentUpstreamJSONModelNotExistChatHint_noop(t *testing.T) {
	in := []byte(`{"code":0,"message":"ok"}`)
	out := augmentUpstreamJSONModelNotExistChatHint(in)
	if string(out) != string(in) {
		t.Fatalf("expected unchanged, got %s", out)
	}
}
