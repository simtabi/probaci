package cli

import (
	"path/filepath"
	"testing"
)

func TestCSV(t *testing.T) {
	got := csv(" a, b ,,c ")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("csv=%v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("csv[%d]=%q want %q", i, got[i], want[i])
		}
	}
	if csv("") != nil {
		t.Fatal("empty csv should be nil")
	}
}

func TestResolveTargetsSingleDir(t *testing.T) {
	g = globals{} // reset package globals
	dir := t.TempDir()
	got, err := resolveTargets([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	abs, _ := filepath.Abs(dir)
	if len(got) != 1 || got[0] != abs {
		t.Fatalf("resolveTargets=%v want [%s]", got, abs)
	}
}

func TestResolveTargetsReposFlag(t *testing.T) {
	g = globals{}
	a, b := t.TempDir(), t.TempDir()
	g.repos = a + "," + b
	got, err := resolveTargets(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 targets, got %v", got)
	}
}

func TestResolveTargetsBothIsError(t *testing.T) {
	g = globals{}
	dir := t.TempDir()
	g.repos = dir
	if _, err := resolveTargets([]string{dir}); err == nil {
		t.Fatal("passing both positional paths and --repos must error")
	}
}

func TestResolveTargetsMissingPath(t *testing.T) {
	g = globals{}
	if _, err := resolveTargets([]string{filepath.Join(t.TempDir(), "nope")}); err == nil {
		t.Fatal("a non-existent target must error")
	}
}
