package klingvideo

import "testing"

func TestPassthroughAllowed(t *testing.T) {
	cfg := AllowlistConfig{}
	if !PassthroughAllowed("videos/text2video", cfg) {
		t.Fatal("videos")
	}
	if !PassthroughAllowed("images/generations", cfg) {
		t.Fatal("images")
	}
	if PassthroughAllowed("../videos/x", cfg) {
		t.Fatal("dotdot")
	}
	if PassthroughAllowed("", cfg) {
		t.Fatal("empty")
	}
}

func TestIsBillableCreate(t *testing.T) {
	if !IsBillableCreate("POST", "videos/text2video") {
		t.Fatal("post video")
	}
	if IsBillableCreate("GET", "videos/text2video/abc") {
		t.Fatal("get")
	}
	if IsBillableCreate("POST", "account/packages") {
		t.Fatal("account")
	}
}

func TestIsTaskQuery(t *testing.T) {
	if !IsTaskQuery("GET", "videos/text2video/task123") {
		t.Fatal("task query")
	}
	if IsTaskQuery("GET", "account/balance") {
		t.Fatal("account")
	}
}

func TestUpstreamRelativePath(t *testing.T) {
	if got := UpstreamRelativePath("videos/text2video"); got != "v1/videos/text2video" {
		t.Fatalf("%q", got)
	}
}
