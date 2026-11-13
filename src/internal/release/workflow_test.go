package release_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// release.yml publishes a release on a pushed version tag, or on request for a tag already
// pushed, and does nothing else: its triggers are exactly those two, and its one job calls the
// shared simtabi/.github go-release.yml (owner, 2026-11-13). A branch push, a schedule, a pull
// request or a release event must never start it. It replaces the dispatch-only, confirm=release-go
// guard of 2026-11-02.
//
// The module has no YAML dependency, so this reads the workflow's indentation: two spaces per
// level, as the file is written. A reformatted file fails here rather than passing unread.
func TestReleaseRunsOnAVersionTagOrOnRequest(t *testing.T) {
	wf := readFile(t, repoRoot(t), ".github", "workflows", "release.yml")

	on := yamlBlock(wf, "", "on")
	if on == "" {
		t.Fatal("release.yml has no top-level on: block")
	}
	if got := strings.Join(yamlKeys(on, "  "), ","); got != "push,workflow_dispatch" {
		t.Fatalf("release.yml triggers [%s], want exactly push (version tags) and workflow_dispatch", got)
	}

	push := yamlBlock(on, "  ", "push")
	if got := strings.Join(yamlKeys(push, "    "), ","); got != "tags" {
		t.Fatalf("release.yml's push filters [%s], want tags only, never a branch", got)
	}
	tags := regexp.MustCompile(`(?m)^      - '([^']*)'\s*$`).FindAllStringSubmatch(yamlBlock(push, "    ", "tags"), -1)
	if len(tags) == 0 {
		t.Fatal("release.yml's push has no quoted tag filters")
	}
	for _, m := range tags {
		if !strings.HasPrefix(m[1], "v[0-9]+.[0-9]+.[0-9]+") {
			t.Errorf("tag filter %q is not a version tag", m[1])
		}
	}

	tagInput := yamlBlock(yamlBlock(yamlBlock(on, "  ", "workflow_dispatch"), "    ", "inputs"), "      ", "tag")
	if !regexp.MustCompile(`(?m)^        required: true\s*$`).MatchString(tagInput) {
		t.Fatal("release.yml has no required tag input on workflow_dispatch")
	}

	jobs := yamlBlock(wf, "", "jobs")
	if got := strings.Join(yamlKeys(jobs, "  "), ","); got != "release" {
		t.Fatalf("release.yml jobs [%s], want the one release job", got)
	}
	job := yamlBlock(jobs, "  ", "release")
	if !regexp.MustCompile(`(?m)^    uses: simtabi/\.github/\.github/workflows/go-release\.yml@[0-9a-f]{40}\b`).MatchString(job) {
		t.Fatalf("the release job must call the shared go-release.yml pinned to a commit:\n%s", job)
	}
	if !strings.Contains(job, "      tag: ${{ inputs.tag || github.ref_name }}\n") {
		t.Error("the release job must pass tag: ${{ inputs.tag || github.ref_name }}")
	}
}

// What the caller hands the shared workflow. The module is github.com/simtabi/probaci/src, which
// Go resolves only through a src/vX.Y.Z tag, so the alias the shared workflow mirrors
// (module-alias-prefix, default src/) must stay on, and the config and changelog it reads must be
// this repository's. Go comes from .go-version, which must say what ci.yml installs, so a release
// is built with the same Go as every pull request.
func TestReleaseInputsMatchTheRepository(t *testing.T) {
	root := repoRoot(t)
	wf := readFile(t, root, ".github", "workflows", "release.yml")
	if !strings.Contains(wf, "uses: simtabi/.github/.github/workflows/go-release.yml@") {
		t.Fatal("release.yml does not call the shared go-release.yml; this guard reads its inputs")
	}
	want := map[string]string{
		"module-alias-prefix": "src/",
		"goreleaser-config":   "src/.goreleaser.yaml",
		"go-version-file":     ".go-version",
		"changelog":           "CHANGELOG.md",
	}
	for input, w := range want {
		m := regexp.MustCompile(`(?m)^\s+` + input + `:\s*['"]?([^'"\s#]*)`).FindStringSubmatch(wf)
		switch {
		case m == nil && input == "go-version-file":
			t.Errorf("release.yml must set go-version-file: %s; the shared default src/go.mod pins Go to 1.26.0", w)
		case m != nil && m[1] != w:
			t.Errorf("release.yml sets %s to %q; this repository needs %q", input, m[1], w)
		}
	}
	for _, f := range []string{"src/.goreleaser.yaml", "src/go.mod", "CHANGELOG.md", ".go-version"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(f))); err != nil {
			t.Errorf("the shared release workflow reads %s, which is missing: %v", f, err)
		}
	}

	goVersion := strings.TrimSpace(readFile(t, root, ".go-version"))
	ci := regexp.MustCompile(`(?m)^\s+go-version:\s*"([^"]+)"`).FindAllStringSubmatch(readFile(t, root, ".github", "workflows", "ci.yml"), -1)
	if len(ci) == 0 {
		t.Fatal("ci.yml installs no go-version; this guard compares .go-version against it")
	}
	for _, m := range ci {
		if m[1] != goVersion {
			t.Errorf("ci.yml installs Go %q but .go-version says %q; a release must build with the Go every pull request tests", m[1], goVersion)
		}
	}
}

// otherChannels are the GoReleaser blocks that publish somewhere other than the GitHub release:
// package managers, images and arbitrary publishers (~/.claude/rules/go.md, owner, 2026-11-13).
// nfpms stays: it builds the .deb/.rpm/.apk files that ship as release assets.
var otherChannels = []string{
	"brews", "homebrew_casks", "scoops", "winget", "chocolateys", "aurs", "aur_sources",
	"snapcrafts", "flatpaks", "nix", "dockers", "dockers_v2", "docker_manifests", "docker_signs",
	"kos", "krews", "npms", "cloudsmiths", "furies", "publishers", "blobs", "uploads", "artifactories",
}

// The GitHub release is the only channel. The Homebrew cask, Scoop manifest and Winget manifest
// this config published until 2026-11-13 are gone from it and must not come back; the shared
// workflow refuses such a config at release time, and this fails first, on the pull request.
func TestGoReleaserPublishesNowhereElse(t *testing.T) {
	cfg := readFile(t, repoRoot(t), "src", ".goreleaser.yaml")
	if !regexp.MustCompile(`(?m)^release:`).MatchString(cfg) {
		t.Fatal("src/.goreleaser.yaml has no release block; the GitHub release is the channel")
	}
	for _, k := range otherChannels {
		if regexp.MustCompile(`(?m)^` + k + `:`).MatchString(cfg) {
			t.Errorf("src/.goreleaser.yaml publishes through %q; binaries go to the GitHub release only", k)
		}
	}
}

// yamlBlock returns the lines nested under the key at the given indent, or "" when it is absent.
func yamlBlock(text, indent, key string) string {
	var out []string
	in := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, indent+key+":") {
			in = true
			continue
		}
		if in {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(line, indent+" ") {
				break
			}
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// yamlKeys lists the mapping keys at exactly the given indent, sorted.
func yamlKeys(text, indent string) []string {
	re := regexp.MustCompile(`^` + indent + `([A-Za-z_][A-Za-z0-9_-]*):`)
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if m := re.FindStringSubmatch(line); m != nil {
			out = append(out, m[1])
		}
	}
	sort.Strings(out)
	return out
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

// Binaries ship on the GitHub release only, so the docs offer the release
// download or a build from source: no `go install` of this module, which would
// serve whatever the Go proxy cached for a tag rather than the released code.
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
			t.Errorf("%s offers %q; the docs offer the GitHub release or a build from source", filepath.Base(f), m)
		}
	}
}
