// Package probaci (module root) embeds the documentation so `probaci docs` can
// render it in-terminal even from an installed binary far from the repo. The
// library API lives in pkg/probaci; this root package only ships assets.
package probaci

import (
	"embed"
	"io/fs"
)

//go:embed README.md docs/*.md docs/tools/*.md
var docsFS embed.FS

// docTopics maps a user-facing topic name (and aliases) to its embedded file.
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
	data, err := fs.ReadFile(docsFS, path)
	if err != nil {
		return "", false
	}
	return string(data), true
}

// DocTopics returns the canonical topic names (deduped), for help/listing.
func DocTopics() []string {
	return []string{"overview", "installation", "configuration", "architecture", "security", "release", "commands"}
}
