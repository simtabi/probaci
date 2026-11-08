package release_test

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// automaticTrigger matches a trigger that starts a workflow without a person
// dispatching it, at any indentation under `on:`.
var automaticTrigger = regexp.MustCompile(`(?m)^[ \t]*(push|schedule|pull_request|pull_request_target|release|workflow_run)\s*:`)

// Go releases are git only until the owner says otherwise, so release.yml must
// never start on its own: no push, schedule, pull_request or release trigger,
// and a dispatch whose first step refuses to publish without confirm=release-go
// on a vX.Y.Z tag. A tag push that started GoReleaser is how an unwanted
// release began elsewhere on 2026-11-02.
func TestReleaseRunsOnlyOnAConfirmedDispatch(t *testing.T) {
	wf := readFile(t, repoRoot(t), ".github", "workflows", "release.yml")

	if m := automaticTrigger.FindString(wf); m != "" {
		t.Errorf("release.yml has an automatic trigger (%q); it must run on workflow_dispatch only", strings.TrimSpace(m))
	}
	for _, want := range []string{"workflow_dispatch:", "confirm:", "required: true", `"${CONFIRM}" != "release-go"`, "refs/tags/v[0-9]*"} {
		if !strings.Contains(wf, want) {
			t.Errorf("release.yml lacks %q: a release needs an explicit, confirmed dispatch on a tag", want)
		}
	}

	// The refusal must be the first step, before anything is checked out,
	// built or published.
	_, steps, ok := strings.Cut(wf, "steps:")
	if !ok {
		t.Fatal("release.yml has no steps")
	}
	first := steps
	if items := regexp.MustCompile(`(?m)^\s*- `).FindAllStringIndex(steps, 2); len(items) == 2 {
		first = steps[:items[1][0]]
	}
	for _, want := range []string{`"release-go"`, "refs/tags/v[0-9]*", "exit 1"} {
		if !strings.Contains(first, want) {
			t.Errorf("the first release step must refuse without confirm=release-go on a vX.Y.Z tag; it lacks %q:\n%s", want, first)
		}
	}
}

// skipDirs are never ours to police: git's own store, dependencies,
// generated output and local tool state.
var skipDirs = map[string]bool{
	".git": true, "vendor": true, "node_modules": true, "build": true, "bin": true,
	".cache": true, ".claude": true, ".probaci": true,
}

// containerFile matches a file that exists only to build or run a container.
var containerFile = regexp.MustCompile(`(?i)^(dockerfile(\..*)?|.*\.dockerfile|\.dockerignore|containerfile|(docker-)?compose(\.[a-z0-9_-]+)?\.ya?ml)$`)

// containerInWorkflow matches a workflow step or GoReleaser block that builds,
// runs or publishes an image.
var containerInWorkflow = regexp.MustCompile(`(?m)uses:\s*['"]?docker/|\bdocker\s+(build|buildx|run|push|compose|login)\b|\bghcr\.io\b|^\s*dockers?(_v2)?\s*:|^\s*docker_manifests\s*:`)

// probaci ships as a binary built from source. No container image is built,
// run or published by the repository itself (probaci's own feature of brokering
// tools through the host's container runtime lives in src/internal and is not a
// file this walk matches), and nothing brings one back without this test
// changing.
func TestNoContainerImage(t *testing.T) {
	root := repoRoot(t)
	visited := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		visited++
		if containerFile.MatchString(d.Name()) {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("%s builds or runs a container; probaci ships a binary built from source", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// The repository held 116 files outside the skipped directories on
	// 2026-11-08; a walk rooted in the wrong place visits a handful.
	if visited < 80 {
		t.Fatalf("visited %d files, want at least 80: the walk is not looking where it should", visited)
	}

	workflows, err := filepath.Glob(filepath.Join(root, ".github", "workflows", "*.y*ml"))
	if err != nil {
		t.Fatal(err)
	}
	// ci, codeql, keepalive, release and scorecard on 2026-11-08.
	if len(workflows) < 5 {
		t.Fatalf("found %d workflow files, want at least 5", len(workflows))
	}
	for _, f := range append(workflows, filepath.Join(root, "src", ".goreleaser.yaml")) {
		if m := containerInWorkflow.FindString(readFile(t, f)); m != "" {
			t.Errorf("%s: %q builds, runs or publishes an image", filepath.Base(f), strings.TrimSpace(m))
		}
	}
	if regexp.MustCompile(`package-ecosystem:\s*['"]?docker`).MatchString(readFile(t, root, ".github", "dependabot.yml")) {
		t.Error("dependabot.yml watches a docker ecosystem")
	}
}

// Releases are on hold, so the docs offer building from source only: no
// `go install` of this module, which would also serve whatever the Go proxy
// cached for a tag rather than the code in the checkout.
func TestDocsOfferNoGoInstall(t *testing.T) {
	root := repoRoot(t)
	files := []string{filepath.Join(root, "README.md"), filepath.Join(root, "CONTRIBUTING.md")}
	var docs []string
	for _, pattern := range []string{"*.md", "tools/*.md"} {
		matches, err := filepath.Glob(filepath.Join(root, "docs", pattern))
		if err != nil {
			t.Fatal(err)
		}
		docs = append(docs, matches...)
	}
	// Six pages on 2026-11-08.
	if len(docs) < 5 {
		t.Fatalf("found %d docs pages, want at least 5", len(docs))
	}
	files = append(files, docs...)
	goInstall := regexp.MustCompile("go install [^\\s`]*github\\.com/")
	for _, f := range files {
		if m := goInstall.FindString(readFile(t, f)); m != "" {
			t.Errorf("%s offers %q; while releases are on hold the docs build from source", filepath.Base(f), m)
		}
	}
}
