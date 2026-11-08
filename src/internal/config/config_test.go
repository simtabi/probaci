package config

import "testing"

func TestDefaultValidates(t *testing.T) {
	if err := Validate(Default()); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
}

func TestMergeOverrides(t *testing.T) {
	base := Default()
	src := Config{Version: SchemaVersion, Project: Project{TestCmd: "go test ./..."}}
	out := merge(base, src)
	if out.Project.TestCmd != "go test ./..." {
		t.Fatalf("override not applied: %q", out.Project.TestCmd)
	}
	// Unset fields keep base values.
	if out.Docker.Backend != BackendAuto {
		t.Fatalf("backend should remain default, got %q", out.Docker.Backend)
	}
}

func TestValidateRejectsUnknownStage(t *testing.T) {
	c := Default()
	c.Stages = append(c.Stages, Stage{Name: "does-not-exist", Enabled: true})
	if err := Validate(c); err == nil {
		t.Fatal("expected error for unknown stage")
	}
}

func TestParseRejectsUnknownFields(t *testing.T) {
	if _, err := Parse([]byte(`{"version":1,"bogus":true}`)); err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestEnabledStagesOrdered(t *testing.T) {
	stages := Default().EnabledStages()
	if len(stages) == 0 || stages[0] != "detect" {
		t.Fatalf("unexpected enabled stages: %v", stages)
	}
}

func TestDockerEnabledAccessors(t *testing.T) {
	if !Default().Docker.IsEnabled() {
		t.Fatal("default docker should be enabled")
	}
	d := Docker{Enabled: BoolPtr(false)}
	if d.IsEnabled() {
		t.Fatal("explicit false should disable")
	}
	if (Docker{}).IsEnabled() == false {
		t.Fatal("unset should default to enabled")
	}
	if Default().Docker.SocketMountAllowed() {
		t.Fatal("default socket mount should be disallowed")
	}
}

// TestConfigCanDisableDocker is the regression test for the merge bug: a config
// layer with "enabled":false must win over the default true.
func TestConfigCanDisableDocker(t *testing.T) {
	base := Default()
	layer, err := Parse([]byte(`{"docker":{"enabled":false}}`))
	if err != nil {
		t.Fatal(err)
	}
	out := merge(base, layer)
	if out.Docker.IsEnabled() {
		t.Fatal("config enabled:false should disable docker after merge")
	}
}
