// Package probaci (module root) embeds the documentation so `probaci docs` can
// render it in-terminal even from an installed binary far from the repo. The
// library API lives in pkg/probaci; this root package only ships assets.
//
// The module lives in src/, and //go:embed cannot reach above the package that
// embeds, so README.md and docs/ (which stay at the repository root, where
// GitHub and the docs site read them) are mirrored into src/embedded/ by
// `make sync-docs`. TestEmbeddedDocsMatchTheRepository fails when the mirror
// drifts from the originals.
package probaci

import (
	"embed"
	"io/fs"
)

//go:embed embedded/README.md embedded/docs/*.md embedded/docs/tools/*.md
var docsFS embed.FS

// embedRoot is the directory, inside docsFS, that mirrors the repository root.
const embedRoot = "embedded/"

// docTopics maps a user-facing topic name (and aliases) to its file, by its
// path from the repository root.
var docTopics = map[string]string{
	"overview":      "README.md",
	"readme":        "README.md",
	"installation":  "docs/installation.md",
	"install":       "docs/installation.md",
	"configuration": "docs/configuration.md",
	"config":        "docs/configuration.md",
	"architecture":  "docs/architecture.md",
	"arch":          "docs/architecture.md",
	"security":      "docs/security.md",
	"release":       "docs/release.md",
	"commands":      "docs/tools/probaci.md",
	"cli":           "docs/tools/probaci.md",
	"tools":         "docs/tools/probaci.md",
	"serve":         "docs/tools/serve.md",
	"ui":            "docs/tools/serve.md",
}

// Doc returns the embedded markdown for a topic (default "overview"), and
// whether the topic is known.
func Doc(topic string) (string, bool) {
	if topic == "" {
		topic = "overview"
	}
	path, ok := docTopics[topic]
	if !ok {
		return "", false
	}
	data, err := fs.ReadFile(docsFS, embedRoot+path)
	if err != nil {
		return "", false
	}
	return string(data), true
}

// DocTopics returns the canonical topic names (deduped), for help/listing.
func DocTopics() []string {
	return []string{"overview", "installation", "configuration", "architecture", "security", "release", "commands", "serve"}
}
