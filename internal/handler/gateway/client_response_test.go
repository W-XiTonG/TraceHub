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

func TestSanitizeClientProxyBody_upstream401NotUserAPIKey(t *testing.T) {
	in := []byte(`{"error":{"message":"Incorrect API key provided: sk-***","type":"invalid_request_error","code":"invalid_api_key"}}`)
	out := SanitizeClientProxyBody(http.StatusUnauthorized, "application/json", in)
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	errObj := m["error"].(map[string]any)
	msg := errObj["message"].(string)
	if strings.Contains(msg, "请检查 API Key") && !strings.Contains(msg, "上游") {
		t.Fatalf("expected upstream channel hint, got %q", msg)
	}
	if !strings.Contains(msg, "TraceHub API Key") {
		t.Fatalf("expected distinction from TraceHub key, got %q", msg)
	}
}

func TestMustBuildOpenAIErrorJSON_401UsesUpstreamAuthMsg(t *testing.T) {
	out := mustBuildOpenAIErrorJSON(http.StatusUnauthorized, 0)
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	msg := m["error"].(map[string]any)["message"].(string)
	if msg != MsgUpstreamAuthFailed {
		t.Fatalf("got %q want %q", msg, MsgUpstreamAuthFailed)
	}
}
