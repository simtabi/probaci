// Package vcs abstracts version control so probaci's "test the committed state,
// not your dirty working tree" guarantee holds on git and on non-git systems
// (Mercurial, Subversion, Perforce, Fossil, Bazaar, Sapling). The CI-platform
// layer is orthogonal to this one.
package vcs

import (
	"context"
	"os"
	"path/filepath"
)

// VCS is a version-control provider.
type VCS interface {
	// Name is the provider's short name (git, hg, svn, …).
	Name() string
	// Detect reports whether dir is (inside) a repository of this kind.
	Detect(dir string) bool
	// IsDirty reports whether the working tree has uncommitted/untracked changes.
	IsDirty(ctx context.Context, dir string) (bool, error)
	// CurrentRev returns the current revision identifier.
	CurrentRev(ctx context.Context, dir string) (string, error)
	// ExportCommitted writes the committed state of dir into destDir. This is
	// what clean-clone runs on, so it must never include the dirty tree.
	ExportCommitted(ctx context.Context, dir, destDir string) error
}

// providers is the ordered detection list. Git first (most common); CVS last
// (detect-only). Each provider degrades gracefully when its binary is absent.
var providers = []VCS{
	git{},
	mercurial{},
	subversion{},
	perforce{},
	fossil{},
	bazaar{},
	sapling{},
	cvs{},
}

// Detect returns the first provider that recognizes dir, or nil if none do.
func Detect(dir string) VCS {
	for _, p := range providers {
		if p.Detect(dir) {
			return p
		}
	}
	return nil
}

// Supported returns the names of every known provider.
func Supported() []string {
	out := make([]string, 0, len(providers))
	for _, p := range providers {
		out = append(out, p.Name())
	}
	return out
}

// hasDir reports whether dir contains a child directory named name (used for
// marker-directory detection like .git, .hg, .svn).
func hasDir(dir, name string) bool {
	info, err := os.Stat(filepath.Join(dir, name))
	return err == nil && info.IsDir()
}

// hasEntry reports whether dir contains a child (file or dir) named name.
func hasEntry(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}
