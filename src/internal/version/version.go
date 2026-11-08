// Package version exposes build metadata, injected via -ldflags at release.
package version

import "runtime/debug"

var (
	// Version is the semantic version (set by GoReleaser via -ldflags).
	Version = "dev"
	// Commit is the git SHA.
	Commit = "none"
	// Date is the build date.
	Date = "unknown"
)

// String returns a human-readable version line. When built without ldflags it
// falls back to VCS info embedded by the Go toolchain.
func String() string {
	v := Version
	if v == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
	}
	return "probaci " + v + " (commit " + Commit + ", built " + Date + ")"
}

// Short returns just the semantic version (e.g. "v0.1.0" or "dev"), resolving
// the embedded build version when ldflags weren't set.
func Short() string {
	if Version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
	}
	return Version
}
