package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFrontendDist_findsDistAlongAncestors(t *testing.T) {
	root := t.TempDir()
	distDir := filepath.Join(root, "myapp", "web", "frontend", "dist")
	if err := os.MkdirAll(distDir, 0o755); err != nil {
		t.Fatal(err)
	}
	idx := filepath.Join(distDir, "index.html")
	if err := os.WriteFile(idx, []byte("<!doctype html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "myapp", "configs", "app.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := resolveFrontendDist(cfgPath)
	if filepath.Clean(got) != filepath.Clean(distDir) {
		t.Fatalf("got %q want %q", got, distDir)
	}
}

func TestResolveFrontendDist_fallbackRelative(t *testing.T) {
	f := filepath.Join(t.TempDir(), "orphan.yaml")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := resolveFrontendDist(f)
	if got != "web/frontend/dist" {
		t.Fatalf("expected default relative path, got %q", got)
	}
}
