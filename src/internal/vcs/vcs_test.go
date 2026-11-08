package vcs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectGit(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	v := Detect(dir)
	if v == nil || v.Name() != "git" {
		t.Fatalf("expected git, got %v", v)
	}
}

func TestDetectMercurialAndNone(t *testing.T) {
	hg := t.TempDir()
	if err := os.MkdirAll(filepath.Join(hg, ".hg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if v := Detect(hg); v == nil || v.Name() != "hg" {
		t.Fatalf("expected hg, got %v", v)
	}
	if v := Detect(t.TempDir()); v != nil {
		t.Fatalf("expected no VCS, got %v", v.Name())
	}
}

func TestSupportedIncludesNonGit(t *testing.T) {
	want := map[string]bool{"git": false, "hg": false, "svn": false, "p4": false, "fossil": false}
	for _, n := range Supported() {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for n, found := range want {
		if !found {
			t.Errorf("provider %q missing from Supported()", n)
		}
	}
}
