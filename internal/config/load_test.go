package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSystemThenUserPrecedence(t *testing.T) {
	dir := t.TempDir()
	sys := filepath.Join(dir, "system.json")
	usr := filepath.Join(dir, "user.json")
	// System sets podman + strict; user overrides backend back to docker.
	if err := os.WriteFile(sys, []byte(`{"docker":{"backend":"podman"},"security":{"verify_images":"strict"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(usr, []byte(`{"docker":{"backend":"docker"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, sources, err := Load(LoadOptions{SystemConfigPath: sys, UserConfigPath: usr})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Docker.Backend != BackendDocker {
		t.Errorf("user should override system backend: got %q", cfg.Docker.Backend)
	}
	if cfg.Security.VerifyImages != "strict" {
		t.Errorf("system verify_images should persist: got %q", cfg.Security.VerifyImages)
	}
	if !hasSource(sources, SourceSystem) || !hasSource(sources, SourceUser) {
		t.Errorf("expected system+user sources, got %v", sources)
	}
}

func TestAtomicWriteRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := Write(path, Default()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(data); err != nil {
		t.Fatalf("written config does not parse: %v", err)
	}
}

func hasSource(s []Source, want Source) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}
