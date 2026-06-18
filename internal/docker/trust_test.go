package docker

import (
	"context"
	"testing"

	"github.com/simtabi/probaci/internal/config"
)

func TestCheckImageStrictBlocksUnpinned(t *testing.T) {
	b := &Broker{sec: config.Security{VerifyImages: "strict"}}
	if _, err := b.CheckImage(context.Background(), "alpine:3"); err == nil {
		t.Fatal("strict mode should reject an unpinned image")
	}
}

func TestCheckImageStrictAllowsPinned(t *testing.T) {
	b := &Broker{sec: config.Security{VerifyImages: "strict"}}
	ref := "alpine@sha256:" + "deadbeef"
	if _, err := b.CheckImage(context.Background(), ref); err != nil {
		t.Fatalf("digest-pinned image should pass strict mode: %v", err)
	}
}

func TestCheckImageStrictAllowList(t *testing.T) {
	b := &Broker{sec: config.Security{VerifyImages: "strict", AllowUnsigned: []string{"alpine"}}}
	if _, err := b.CheckImage(context.Background(), "alpine:3"); err != nil {
		t.Fatalf("allow-listed image should pass strict mode: %v", err)
	}
}

func TestRegistryMirrorRewrite(t *testing.T) {
	b := &Broker{sec: config.Security{RegistryMirror: "mirror.example.com"}}
	cases := map[string]string{
		"golang":                 "mirror.example.com/golang",        // bare official
		"aquasec/trivy":          "mirror.example.com/aquasec/trivy", // org/name
		"ghcr.io/astral-sh/ruff": "ghcr.io/astral-sh/ruff",           // already has a registry host
		"mirror.example.com/x/y": "mirror.example.com/x/y",           // already mirrored
		"localhost:5000/x":       "localhost:5000/x",                 // host with port
	}
	for in, want := range cases {
		if got := b.image(in); got != want {
			t.Errorf("image(%q)=%q want %q", in, got, want)
		}
	}
	// No mirror configured: passthrough.
	plain := &Broker{}
	if got := plain.image("golang"); got != "golang" {
		t.Errorf("no-mirror passthrough failed: %q", got)
	}
}

func TestCheckImageAdvisoryNeverBlocks(t *testing.T) {
	b := &Broker{sec: config.Security{VerifyImages: "advisory"}}
	note, err := b.CheckImage(context.Background(), "alpine:3")
	if err != nil {
		t.Fatalf("advisory mode must not block: %v", err)
	}
	if note == "" {
		t.Fatal("advisory mode should return a note for an unpinned image")
	}
}
