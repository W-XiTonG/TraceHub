package klingvideo

import (
	"encoding/json"
	"testing"
)

func TestPrepareKlingVideoCreateBody_text(t *testing.T) {
	in := []byte(`{"model":"kling-v2.6-pro","prompt":"a cat","seconds":"10","size":"1280x720"}`)
	kind, body, err := PrepareKlingVideoCreateBody(in)
	if err != nil {
		t.Fatal(err)
	}
	if kind != CreateText2Video {
		t.Fatalf("kind %q", kind)
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	if m["model_name"] != "kling-v2.6-pro" {
		t.Fatalf("model_name %v", m["model_name"])
	}
	if m["duration"] != float64(10) && m["duration"] != 10 {
		t.Fatalf("duration %v", m["duration"])
	}
	if m["aspect_ratio"] != "16:9" {
		t.Fatalf("aspect_ratio %v", m["aspect_ratio"])
	}
}

func TestPrepareKlingVideoCreateBody_image(t *testing.T) {
	in := []byte(`{"model":"kling-v2.6-pro","prompt":"move","image_url":"https://example.com/a.png"}`)
	kind, _, err := PrepareKlingVideoCreateBody(in)
	if err != nil {
		t.Fatal(err)
	}
	if kind != CreateImage2Video {
		t.Fatalf("kind %q", kind)
	}
}

func TestBearerToken_and_ParseCredentials(t *testing.T) {
	_, _, err := ParseCredentials("bad")
	if err == nil {
		t.Fatal("expected error")
	}
	tok, err := BearerToken("ak-test:sk-test-secret")
	if err != nil {
		t.Fatal(err)
	}
	if tok == "" {
		t.Fatal("empty token")
	}
	tok2, err := BearerToken("ak-test:sk-test-secret")
	if err != nil || tok2 != tok {
		t.Fatal("expected cache hit")
	}
}

func TestNormalizeResponseBody_klingData(t *testing.T) {
	in := []byte(`{"code":0,"data":{"task_id":"t1","task_status":"succeed","task_result":{"videos":[{"url":"https://cdn/x.mp4","duration":"5"}]}}}`)
	out := NormalizeResponseBody(in)
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if m["id"] != "t1" {
		t.Fatalf("id %v", m["id"])
	}
	if m["status"] != "completed" {
		t.Fatalf("status %v", m["status"])
	}
	if m["video_url"] != "https://cdn/x.mp4" {
		t.Fatalf("video_url %v", m["video_url"])
	}
}

func TestCreateKind_QueryPath(t *testing.T) {
	if p := CreateText2Video.QueryPath("abc"); p != "v1/videos/text2video/abc" {
		t.Fatal(p)
	}
	if p := CreateImage2Video.QueryPath("abc"); p != "v1/videos/image2video/abc" {
		t.Fatal(p)
	}
}
