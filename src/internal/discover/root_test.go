package discover

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootFindsProbaciJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "probaci.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, ok := Root(sub)
	if !ok {
		t.Fatal("expected to find root")
	}
	// macOS /var symlinks to /private/var; compare resolved paths.
	wantResolved, _ := filepath.EvalSymlinks(root)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != wantResolved {
		t.Fatalf("Root=%s want %s", gotResolved, wantResolved)
	}
}

func TestRootFallsBackToVCS(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, ok := Root(sub)
	if !ok {
		t.Fatal("expected VCS fallback to find root")
	}
	wantResolved, _ := filepath.EvalSymlinks(root)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != wantResolved {
		t.Fatalf("Root=%s want %s", gotResolved, wantResolved)
	}
}

func TestRootNotFound(t *testing.T) {
	dir := t.TempDir()
	if _, ok := Root(dir); ok {
		t.Fatal("expected no root in an empty temp dir")
	}
}
