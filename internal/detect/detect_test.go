package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectGoAndCommands(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "go.mod")
	p := DetectProject(dir)
	if len(p.Languages) != 1 || p.Languages[0].Name != "go" {
		t.Fatalf("expected go, got %+v", p.Languages)
	}
	l := p.Languages[0]
	if l.InstallCmd == "" || l.TestCmd == "" || l.LintTool != "golangci-lint" || l.AuditTool != "govulncheck" {
		t.Fatalf("unexpected go language config: %+v", l)
	}
}

func TestDetectNodePackageManager(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json")
	write(t, dir, "pnpm-lock.yaml")
	p := DetectProject(dir)
	if len(p.Languages) == 0 || p.Languages[0].Name != "node" {
		t.Fatalf("expected node, got %+v", p.Languages)
	}
	if p.Languages[0].InstallCmd != "pnpm install --frozen-lockfile" {
		t.Fatalf("expected pnpm install, got %q", p.Languages[0].InstallCmd)
	}
	if p.NoLock {
		t.Fatal("lockfile present, NoLock should be false")
	}
}

func TestDetectNodeNoLock(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json")
	p := DetectProject(dir)
	if !p.NoLock {
		t.Fatal("missing lockfile should set NoLock")
	}
}

func TestDetectPolyglot(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "go.mod")
	write(t, dir, "pyproject.toml")
	p := DetectProject(dir)
	if len(p.Languages) != 2 {
		t.Fatalf("expected go+python, got %+v", p.Languages)
	}
}

func TestDetectEmpty(t *testing.T) {
	if p := DetectProject(t.TempDir()); len(p.Languages) != 0 {
		t.Fatalf("empty dir should detect nothing, got %+v", p.Languages)
	}
}
