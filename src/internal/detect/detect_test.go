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

// mkdirWrite creates dir/sub and writes name inside it.
func mkdirWrite(t *testing.T, dir, sub, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, sub), name)
}

// TestDetectGoModuleInSrc covers the src/ layout every Simtabi Go repository
// uses: go.mod lives in src/, not at the root. The commands and the module
// directory must point at src/, or `go test ./...` at the root finds no
// packages and passes.
func TestDetectGoModuleInSrc(t *testing.T) {
	dir := t.TempDir()
	mkdirWrite(t, dir, "src", "go.mod")
	p := DetectProject(dir)
	if len(p.Languages) != 1 || p.Languages[0].Name != "go" {
		t.Fatalf("expected go from src/go.mod, got %+v", p.Languages)
	}
	l := p.Languages[0]
	if l.Dir != "src" {
		t.Errorf("Dir = %q, want %q", l.Dir, "src")
	}
	if l.InstallCmd != "go -C src mod download" || l.TestCmd != "go -C src test ./..." {
		t.Errorf("commands must run in src/: install=%q test=%q", l.InstallCmd, l.TestCmd)
	}
}

// TestDetectGoModuleOneLevelDown finds a module in any immediate subdirectory,
// not only src/.
func TestDetectGoModuleOneLevelDown(t *testing.T) {
	dir := t.TempDir()
	mkdirWrite(t, dir, "service", "go.mod")
	p := DetectProject(dir)
	if len(p.Languages) != 1 || p.Languages[0].Dir != "service" {
		t.Fatalf("expected go in service/, got %+v", p.Languages)
	}
}

// TestDetectGoModulePrefersRootThenSrc: a root go.mod wins over any nested
// one, and src/ wins over other subdirectories.
func TestDetectGoModulePrefersRootThenSrc(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod")
	mkdirWrite(t, root, "src", "go.mod")
	if p := DetectProject(root); len(p.Languages) != 1 || p.Languages[0].Dir != "" ||
		p.Languages[0].TestCmd != "go test ./..." {
		t.Fatalf("root go.mod should win, got %+v", p.Languages)
	}

	nested := t.TempDir()
	mkdirWrite(t, nested, "aaa", "go.mod")
	mkdirWrite(t, nested, "src", "go.mod")
	if p := DetectProject(nested); len(p.Languages) != 1 || p.Languages[0].Dir != "src" {
		t.Fatalf("src/ should win over other subdirectories, got %+v", p.Languages)
	}
}

// TestDetectGoModuleIgnoresNonModuleDirs: a go.mod under vendor/, testdata/,
// node_modules/ or a hidden or underscore directory is not the project's
// module (Go itself ignores testdata/, and "." and "_" prefixed directories).
func TestDetectGoModuleIgnoresNonModuleDirs(t *testing.T) {
	dir := t.TempDir()
	skipped := []string{"vendor", "testdata", "node_modules", ".cache", "_parked"}
	for _, sub := range skipped {
		mkdirWrite(t, dir, sub, "go.mod")
	}
	// Guard against an under-asserting setup: every fixture must exist.
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != len(skipped) {
		t.Fatalf("fixture has %d dirs, want %d (err %v)", len(entries), len(skipped), err)
	}
	if p := DetectProject(dir); len(p.Languages) != 0 {
		t.Fatalf("go.mod in excluded dirs must not count, got %+v", p.Languages)
	}
}

// TestDetectGoModuleNotTwoLevelsDown keeps the search to one level, so a
// fixture module deep in the tree is never mistaken for the project.
func TestDetectGoModuleNotTwoLevelsDown(t *testing.T) {
	dir := t.TempDir()
	mkdirWrite(t, dir, filepath.Join("tools", "gen"), "go.mod")
	if p := DetectProject(dir); len(p.Languages) != 0 {
		t.Fatalf("two levels down must not count, got %+v", p.Languages)
	}
}

// TestDetectSelf: probaci's own repository keeps its module in src/ and must
// detect as Go. This test runs in src/internal/detect, three levels below
// the repository root.
func TestDetectSelf(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(root, "src"), "go.mod") {
		t.Fatalf("expected probaci's module at %s/src/go.mod", root)
	}
	p := DetectProject(root)
	if len(p.Languages) == 0 || p.Languages[0].Name != "go" || p.Languages[0].Dir != "src" {
		t.Fatalf("probaci itself should detect as go in src/, got %+v", p.Languages)
	}
}
