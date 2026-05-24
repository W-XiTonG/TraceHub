package relay

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStreamUpstreamContext_survivesParentCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	streamCtx := StreamUpstreamContext(ctx)
	cancel()
	select {
	case <-streamCtx.Done():
		t.Fatal("stream context should not be canceled when parent is canceled")
	default:
	}
}

func TestForwardOpenAIImageGenerations_survivesAttemptContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(40 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"url":"https://example.com/a.png"}]}`)
	}))
	defer srv.Close()

	attemptCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	resp, err := ForwardOpenAIImageGenerations(attemptCtx, srv.URL, "test-key", []byte(`{"model":"gpt-image-2","prompt":"cat"}`))
	if err != nil {
		t.Fatalf("forward: %v", err)
	}
	defer DrainBody(resp.Body)
	cancel()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(raw), "example.com") {
		t.Fatalf("expected full response, got %q", raw)
	}
}

func TestForwardOpenAIChat_streamSurvivesAttemptContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"chunk\":1}\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(30 * time.Millisecond)
		fmt.Fprint(w, "data: {\"chunk\":2}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	attemptCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	resp, err := ForwardOpenAIChat(attemptCtx, srv.URL, "test-key", []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}],"stream":true}`), true)
	if err != nil {
		t.Fatalf("forward: %v", err)
	}
	defer DrainBody(resp.Body)
	cancel()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, `"chunk":2`) {
		t.Fatalf("expected full SSE stream, got %q", body)
	}
}
