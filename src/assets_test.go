package probaci

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocLookup(t *testing.T) {
	for _, topic := range DocTopics() {
		if _, ok := Doc(topic); !ok {
			t.Errorf("topic %q should resolve to an embedded doc", topic)
		}
	}
	// Default + aliases.
	if md, ok := Doc(""); !ok || md == "" {
		t.Fatal("empty topic should default to overview")
	}
	if _, ok := Doc("config"); !ok {
		t.Fatal("alias 'config' should resolve")
	}
	if _, ok := Doc("nope"); ok {
		t.Fatal("unknown topic should not resolve")
	}
}

// The embedded docs are a copy of README.md and docs/ (the module in src/
// cannot embed files above it), so the copy must match the originals file for
// file, in both directions. `make sync-docs` refreshes it.
func TestEmbeddedDocsMatchTheRepository(t *testing.T) {
	originals := []string{"README.md"}
	for _, pattern := range []string{"docs/*.md", "docs/tools/*.md"} {
		matches, err := filepath.Glob(filepath.Join("..", filepath.FromSlash(pattern)))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range matches {
			rel, _ := filepath.Rel("..", m)
			originals = append(originals, filepath.ToSlash(rel))
		}
	}
	// README plus six docs pages on 2026-11-07; a glob looking in the wrong
	// place finds none and would pass an empty mirror.
	if len(originals) < 5 {
		t.Fatalf("found %d docs files at the repository root, want at least 5", len(originals))
	}

	embedded := map[string]bool{}
	err := fs.WalkDir(docsFS, "embedded", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			embedded[strings.TrimPrefix(path, embedRoot)] = true
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, rel := range originals {
		want, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		got, err := fs.ReadFile(docsFS, embedRoot+rel)
		if err != nil {
			t.Errorf("%s is not embedded; run `make sync-docs`", rel)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("src/embedded/%s differs from %s; run `make sync-docs`", rel, rel)
		}
		delete(embedded, rel)
	}
	for rel := range embedded {
		t.Errorf("src/embedded/%s has no original at the repository root; run `make sync-docs`", rel)
	}
}
