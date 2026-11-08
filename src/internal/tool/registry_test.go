package tool

import (
	"strings"
	"testing"

	"github.com/simtabi/probaci/internal/config"
)

func TestResolveBuiltin(t *testing.T) {
	r := New(nil)
	tl, err := r.Resolve("gitleaks")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tl.Ref(), "gitleaks") {
		t.Fatalf("unexpected ref %q", tl.Ref())
	}
}

func TestResolveUnknown(t *testing.T) {
	if _, err := New(nil).Resolve("nope"); err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestOverrideAndDigestRef(t *testing.T) {
	r := New(map[string]config.Tool{
		"gitleaks": {Tag: "v8.18.0", Digest: "sha256:abc"},
	})
	tl, err := r.Resolve("gitleaks")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(tl.Ref(), "@sha256:abc") {
		t.Fatalf("digest should win in Ref(): %q", tl.Ref())
	}
}

func TestUserDefinedTool(t *testing.T) {
	r := New(map[string]config.Tool{
		"mytool": {Image: "example/mytool", Tag: "1"},
	})
	tl, err := r.Resolve("mytool")
	if err != nil {
		t.Fatal(err)
	}
	if tl.Ref() != "example/mytool:1" {
		t.Fatalf("unexpected ref %q", tl.Ref())
	}
}
