package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateLegacy(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "ci-local.config.json")
	body := `{
	  "install_cmd": "npm ci",
	  "test_cmd": "npm test",
	  "secrets_file": ".secrets",
	  "stages": ["actionlint", "secrets", "act"]
	}`
	if err := os.WriteFile(legacy, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Migrate(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project.InstallCmd != "npm ci" || cfg.Project.TestCmd != "npm test" {
		t.Fatalf("commands not migrated: %+v", cfg.Project)
	}
	if cfg.Secrets != ".secrets" {
		t.Fatalf("secrets_file not migrated: %q", cfg.Secrets)
	}
	enabled := map[string]bool{}
	for _, s := range cfg.Stages {
		enabled[s.Name] = s.Enabled
	}
	// actionlint->workflow-lint, act->workflow-run, secrets->secrets.
	for _, want := range []string{"workflow-lint", "secrets", "workflow-run"} {
		if !enabled[want] {
			t.Errorf("stage %q should be enabled after migration", want)
		}
	}
	if enabled["lint"] {
		t.Error("lint was not in the legacy stages; should be disabled")
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("migrated config invalid: %v", err)
	}
}
