package gateway

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestSanitizeClientProxyBody_openAIUpstream(t *testing.T) {
	in := []byte(`{"error":{"message":"The model does not exist. Request id: 021234567890abcdef","type":"invalid_request_error"}}`)
	out := SanitizeClientProxyBody(http.StatusBadRequest, "application/json", in)
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	errObj := m["error"].(map[string]any)
	msg := errObj["message"].(string)
	if strings.Contains(strings.ToLower(msg), "request id") {
		t.Fatalf("leaked request id: %q", msg)
	}
	if !strings.Contains(msg, "模型不可用") {
		t.Fatalf("expected platform message, got %q", msg)
	}
}

func TestSanitizeClientProxyBody_legacy20012(t *testing.T) {
	in := []byte(`{"code":20012,"message":"Model does not exist. Please check it carefully.","data":null}`)
	out := SanitizeClientProxyBody(http.StatusBadRequest, "application/json", in)
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	errObj := m["error"].(map[string]any)
	msg := errObj["message"].(string)
	if !strings.Contains(msg, "对话补全") {
		t.Fatalf("expected chat hint, got %q", msg)
	}
}

func TestSanitizeClientProxyBody_successPassthrough(t *testing.T) {
	in := []byte(`{"id":"vid_1","status":"queued"}`)
	out := SanitizeClientProxyBody(http.StatusOK, "application/json", in)
	if string(out) != string(in) {
		t.Fatalf("expected unchanged success body, got %s", out)
	}
}
