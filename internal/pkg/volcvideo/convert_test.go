package volcvideo

import (
	"encoding/json"
	"testing"
)

func TestBuildArkCreateBody(t *testing.T) {
	in := []byte(`{"model":"doubao-seedance","prompt":"a cat","seconds":"8","size":"1280x720"}`)
	out, err := BuildArkCreateBody(in)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if m["duration"] == nil {
		t.Fatal("want duration")
	}
	if m["resolution"] != "720p" {
		t.Fatalf("resolution %v", m["resolution"])
	}
}

func TestBuildArkCreateBody_1080pFromSize(t *testing.T) {
	in := []byte(`{"model":"doubao-seedance-2-0-260128","prompt":"a cat","seconds":"8","size":"1920x1080"}`)
	out, err := BuildArkCreateBody(in)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if m["resolution"] != "1080p" {
		t.Fatalf("resolution %v", m["resolution"])
	}
}

func TestPrepareVolcVideoCreateBody_nativePassthrough(t *testing.T) {
	in := []byte(`{
		"model":"doubao-seedance-2-0-260128",
		"content":[
			{"type":"text","text":"hello seedance"},
			{"type":"image_url","image_url":{"url":"https://example.com/a.jpg"},"role":"reference_image"}
		],
		"generate_audio":true,
		"ratio":"16:9",
		"duration":11,
		"watermark":false
	}`)
	out, err := PrepareVolcVideoCreateBody(in)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if m["generate_audio"] != true {
		t.Fatalf("generate_audio %v", m["generate_audio"])
	}
	content, ok := m["content"].([]any)
	if !ok || len(content) != 2 {
		t.Fatalf("content len %v", m["content"])
	}
	item1, _ := content[1].(map[string]any)
	if item1["role"] != "reference_image" {
		t.Fatalf("role %v", item1["role"])
	}
}

func TestBuildArk3DCreateBody_requiresImage(t *testing.T) {
	in := []byte(`{"model":"doubao-seed3d-2-0-260328","prompt":"a cube","seconds":"8","size":"1280x720"}`)
	_, err := PrepareVolcVideoCreateBody(in)
	if err == nil {
		t.Fatal("want error without reference image")
	}
}

func TestBuildArk3DCreateBody_fromImageURL(t *testing.T) {
	in := []byte(`{
		"model":"doubao-seed3d-2-0-260328",
		"prompt":"a cube --fileformat glb",
		"image_url":"https://example.com/ref.png"
	}`)
	out, err := PrepareVolcVideoCreateBody(in)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	content, ok := m["content"].([]any)
	if !ok || len(content) != 2 {
		t.Fatalf("content %v", m["content"])
	}
	t0, _ := content[0].(map[string]any)
	if t0["text"] != "a cube --fileformat glb" {
		t.Fatalf("text %v", t0["text"])
	}
	t1, _ := content[1].(map[string]any)
	iu, _ := t1["image_url"].(map[string]any)
	if iu["url"] != "https://example.com/ref.png" {
		t.Fatalf("url %v", iu["url"])
	}
	for _, k := range []string{"image_data", "prompt"} {
		if _, ok := m[k]; ok {
			t.Fatalf("seed3d must not send top-level %s", k)
		}
	}
}

func TestPrepareVolcVideoCreateBody_seed3dFromContentImageURL(t *testing.T) {
	in := []byte(`{
		"model":"doubao-seed3d-2-0-260328",
		"content":[
			{"type":"text","text":"a cube"},
			{"type":"image_url","image_url":{"url":"https://example.com/a.jpg"}}
		],
		"duration":8,
		"resolution":"720p"
	}`)
	out, err := PrepareVolcVideoCreateBody(in)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	content, ok := m["content"].([]any)
	if !ok || len(content) != 2 {
		t.Fatalf("content %v", m["content"])
	}
	t1, _ := content[1].(map[string]any)
	iu, _ := t1["image_url"].(map[string]any)
	if iu["url"] != "https://example.com/a.jpg" {
		t.Fatalf("url %v", iu["url"])
	}
	if m["duration"] != float64(8) {
		t.Fatalf("duration %v", m["duration"])
	}
}

func TestNormalizeResponseBody(t *testing.T) {
	in := []byte(`{"task_id":"t1","status":"succeeded","duration":8}`)
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
}
