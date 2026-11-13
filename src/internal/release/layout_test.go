// Package release_test guards how probaci is built and released. It holds
// tests only: the workflows are YAML and build/ is a directory, so there is no
// Go code here to test, just a policy to pin.
package release_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// repoRoot walks up from the test's directory to the repository root: the
// directory holding src/go.mod. The module lives in src/, so the root is its
// parent, not the directory holding go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "src", "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("src/go.mod not found above the test directory")
		}
		dir = parent
	}
}

func readFile(t *testing.T, parts ...string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// buildInputs are the files build/ may hold besides the OS folders.
var buildInputs = map[string]bool{"targets.txt": true, "nfpm.yaml": true, "checksums.txt": true}

// osFolders are the folders build/ may hold: one per OS, named by GOOS except
// darwin, which is macos. archives/ is written by `make dist`.
var osFolders = map[string]bool{
	"macos": true, "linux": true, "windows": true, "freebsd": true, "openbsd": true,
	"netbsd": true, "dragonfly": true, "solaris": true, "illumos": true, "aix": true,
	"plan9": true, "android": true, "ios": true, "js": true, "wasip1": true,
	"archives": true,
}

// build/ holds one folder per OS and nothing else: no build/dist/, no darwin/,
// no per-target subfolders and no tool intermediates. `make check-build` runs
// this after every build, and CI runs it after `make cross`.
func TestBuildLayout(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "build")
	if problems := buildLayoutProblems(t, dir); len(problems) > 0 {
		t.Errorf("build/ breaks the layout (one folder per OS, see the Makefile):\n  %s", strings.Join(problems, "\n  "))
	}
	n, floor := buildEntries(t, dir), buildFloor(t)
	if n < floor {
		t.Errorf("inspected %d entries in build/, expected at least %d (BUILD_LAYOUT_MIN): the check read less than the build wrote", n, floor)
	}
	t.Logf("inspected %d entries in build/ (floor %d)", n, floor)
}

// buildFloor is the fewest entries build/ must hold: BUILD_LAYOUT_MIN, which the
// Makefile sets to what the build that just ran wrote, else 1 (targets.txt).
func buildFloor(t *testing.T) int {
	t.Helper()
	raw := os.Getenv("BUILD_LAYOUT_MIN")
	if raw == "" {
		return 1
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		t.Fatalf("BUILD_LAYOUT_MIN=%q is not a positive number", raw)
	}
	return n
}

// buildEntries counts what the layout check inspected: the files at the root of
// build/ and the files in each OS folder. Folders themselves do not count.
func buildEntries(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	n := 0
	for _, e := range entries {
		switch {
		case e.Name() == ".DS_Store":
		case !e.IsDir():
			n++
		case osFolders[e.Name()]:
			inner, err := os.ReadDir(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range inner {
				if !f.IsDir() && f.Name() != ".DS_Store" {
					n++
				}
			}
		}
	}
	return n
}

// The count is what the floor stands on, so it has to count what is there.
func TestBuildEntriesCountsWhatItInspected(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{
		"targets.txt", "checksums.txt", ".DS_Store",
		"macos/app_macos_arm64", "linux/app_linux_amd64", "archives/app_linux_amd64.tar.gz",
	} {
		p := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := buildEntries(t, dir); got != 5 {
		t.Errorf("buildEntries counted %d, want 5", got)
	}
}

// The layout check has to be able to fail: each of these trees breaks it.
func TestBuildLayoutRejectsStrayOutput(t *testing.T) {
	cases := map[string][]string{
		"a dist folder":        {"dist/probaci_linux_amd64"},
		"a darwin folder":      {"darwin/probaci_darwin_arm64"},
		"a per-target folder":  {"linux/probaci_linux_amd64/probaci"},
		"a loose binary":       {"probaci"},
		"goreleaser metadata":  {"artifacts.json"},
		"a nested macos build": {"macos/universal/probaci"},
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range append([]string{"targets.txt"}, files...) {
				p := filepath.Join(dir, filepath.FromSlash(f))
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if len(buildLayoutProblems(t, dir)) == 0 {
				t.Errorf("%v passed the layout check", files)
			}
		})
	}
}

func buildLayoutProblems(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "targets.txt")); err != nil {
		t.Fatalf("%s has no targets.txt: the check is not looking at build/", dir)
	}
	var problems []string
	for _, e := range entries {
		name := e.Name()
		switch {
		case name == ".DS_Store":
		case !e.IsDir() && buildInputs[name]:
		case e.IsDir() && osFolders[name]:
			inner, err := os.ReadDir(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range inner {
				if f.IsDir() {
					problems = append(problems, name+"/"+f.Name()+"/ is a subfolder; an OS folder holds binaries only")
				}
			}
		case e.IsDir():
			problems = append(problems, name+"/ is not an OS folder (darwin is written macos; there is no dist/)")
		default:
			problems = append(problems, name+" belongs in an OS folder, or nowhere")
		}
	}
	return problems
}
