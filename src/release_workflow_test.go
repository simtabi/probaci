package probaci

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Go releases are git only until the owner says otherwise, so release.yml must
// never start on its own: no push trigger of any kind, and a dispatch that
// refuses to publish without confirm=release-go. A tag push that started
// GoReleaser is how an unwanted release began elsewhere on 2026-11-02.
func TestReleaseRunsOnlyOnAConfirmedDispatch(t *testing.T) {
	wf, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if regexp.MustCompile(`(?m)^[ \t]*(push|schedule|pull_request|release)\s*:`).Match(wf) {
		t.Error("release.yml has an automatic trigger; it must run on workflow_dispatch only")
	}
	for _, want := range []string{"workflow_dispatch:", "confirm:", `"${CONFIRM}" != "release-go"`, "refs/tags/v[0-9]*"} {
		if !strings.Contains(string(wf), want) {
			t.Errorf("release.yml lacks %q: a release needs an explicit, confirmed dispatch on a tag", want)
		}
	}
}
