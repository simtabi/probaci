// Package discover implements ddev-style repository-root discovery: when the
// user runs probaci from a subdirectory without naming a path, walk up to the
// nearest project root so commands "just work" from anywhere in the tree.
package discover

import (
	"os"
	"path/filepath"
)

// markers identify a project/repository root. probaci.json wins; otherwise the
// nearest VCS marker.
var configMarkers = []string{"probaci.json"}
var vcsMarkers = []string{".git", ".hg", ".svn", ".bzr", ".fslckout", ".sl"}

// Root walks up from start to the nearest directory containing a probaci.json,
// else the nearest VCS marker. It returns (root, true) on success, or
// (start, false) if none is found (callers then fall back to start itself).
func Root(start string) (string, bool) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return start, false
	}
	// First pass: prefer an explicit probaci.json (project config wins).
	if dir, ok := walkUp(abs, configMarkers); ok {
		return dir, true
	}
	// Second pass: fall back to the VCS root.
	if dir, ok := walkUp(abs, vcsMarkers); ok {
		return dir, true
	}
	return abs, false
}

func walkUp(dir string, markers []string) (string, bool) {
	for {
		for _, m := range markers {
			if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
				return dir, true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir { // reached filesystem root
			return "", false
		}
		dir = parent
	}
}
