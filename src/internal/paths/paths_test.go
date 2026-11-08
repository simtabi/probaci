package paths

import (
	"path/filepath"
	"testing"
)

func TestResolveHonorsProbaciHome(t *testing.T) {
	t.Setenv(EnvHome, "/tmp/custom-home")
	l := Resolve()
	if l.Home != "/tmp/custom-home" {
		t.Fatalf("Home=%q want /tmp/custom-home", l.Home)
	}
	if l.Config != filepath.Join("/tmp/custom-home", "config.json") {
		t.Fatalf("Config=%q", l.Config)
	}
	if l.Lock == "" {
		t.Fatal("Lock path should be set")
	}
}

func TestResolveHonorsXDG(t *testing.T) {
	t.Setenv(EnvHome, "")
	t.Setenv(EnvXDGConfig, "/tmp/xdg")
	l := Resolve()
	if l.Home != filepath.Join("/tmp/xdg", "probaci") {
		t.Fatalf("Home=%q want /tmp/xdg/probaci", l.Home)
	}
}

func TestSystemConfigOverride(t *testing.T) {
	t.Setenv("PROBACI_SYSTEM_DIR", "/tmp/sys")
	l := Resolve()
	if l.SystemConfig != filepath.Join("/tmp/sys", "config.json") {
		t.Fatalf("SystemConfig=%q", l.SystemConfig)
	}
}
