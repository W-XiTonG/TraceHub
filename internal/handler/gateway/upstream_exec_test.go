package gateway

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"tracehub/internal/gateway/router"
)

func TestHTTPStreamResponseToUpstreamResult_transportError(t *testing.T) {
	res, err := HTTPStreamResponseToUpstreamResult(nil, io.EOF)
	if err == nil {
		t.Fatal("expected error")
	}
	if res == nil || res.TransportErr == nil {
		t.Fatal("expected transport err in result")
	}
}

func TestHTTPStreamResponseToUpstreamResult_4xxNoStream(t *testing.T) {
	resp := &http.Response{
		StatusCode: 401,
		Body:       io.NopCloser(strings.NewReader(`{"error":"unauthorized"}`)),
		Header:     make(http.Header),
	}
	res, err := HTTPStreamResponseToUpstreamResult(resp, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.StreamResponse != nil {
		t.Fatal("expected no stream response on 4xx")
	}
	if res.StatusCode != 401 || len(res.Body) == 0 {
		t.Fatalf("body=%q status=%d", res.Body, res.StatusCode)
	}
	if !router.IsRetryableUpstream(res.StatusCode, nil) {
		// 401 should not retry
	} else {
		t.Fatal("401 should not be retryable")
	}
}

func TestHTTPStreamResponseToUpstreamResult_200KeepsStream(t *testing.T) {
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("data: {}\n\n")),
		Header:     make(http.Header),
	}
	res, err := HTTPStreamResponseToUpstreamResult(resp, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.StreamResponse == nil {
		t.Fatal("expected stream response on 200")
	}
	if len(res.Body) != 0 {
		t.Fatal("body should not be read on 200")
	}
}

func TestHTTPStreamResponseToUpstreamResult_503Retryable(t *testing.T) {
	resp := &http.Response{
		StatusCode: 503,
		Body:       io.NopCloser(strings.NewReader("unavailable")),
		Header:     make(http.Header),
	}
	res, err := HTTPStreamResponseToUpstreamResult(resp, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !router.IsRetryableUpstream(res.StatusCode, nil) {
		t.Fatal("503 should be retryable")
	}
}
