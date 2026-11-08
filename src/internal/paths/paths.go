// Package paths resolves probaci's user configuration home and its
// subdirectories in a consistent, overridable way across Linux, macOS, and
// Windows.
//
// Resolution order for the base directory (highest priority first):
//
//	$PROBACI_HOME
//	$XDG_CONFIG_HOME/probaci
//	~/.config/probaci            (Linux & macOS)
//	%AppData%\probaci            (Windows)
//
// XDG is honored on every platform so the "~/.config/probaci" layout stays
// consistent cross-OS. If none of these can be resolved the caller falls back
// to embedded built-in defaults — nothing here is required to exist on disk.
package paths

import (
	"os"
	"path/filepath"
	"runtime"
)

// Layout holds the resolved set of directories and well-known files that make
// up a probaci user home.
type Layout struct {
	// Home is the base configuration directory (e.g. ~/.config/probaci).
	Home string
	// SystemConfig is the read-only machine-wide config an admin may set
	// (/etc/probaci/config.json, %ProgramData%\probaci\config.json). It sits
	// below the user config in precedence and may not exist.
	SystemConfig string
	// Lock is the advisory lock file guarding mutating config operations.
	Lock string
	// Config is the user-global config file (Home/config.json).
	Config string
	// Schema is the exported JSON Schema (Home/config.schema.json).
	Schema string
	// Tools is the optional user tool-registry override (Home/tools.json).
	Tools string
	// Logs is the rotating-log directory (Home/logs).
	Logs string
	// Cache is the cache directory (Home/cache).
	Cache string
	// Secrets is the 0600 secrets directory (Home/secrets).
	Secrets string
}

const (
	// EnvHome overrides the entire base directory.
	EnvHome = "PROBACI_HOME"
	// EnvXDGConfig is the standard XDG config base.
	EnvXDGConfig = "XDG_CONFIG_HOME"

	configName = "config.json"
	schemaName = "config.schema.json"
	toolsName  = "tools.json"
	logsName   = "logs"
	cacheName  = "cache"
	secretName = "secrets"

	appDir = "probaci"
)

// Resolve computes the user home Layout. It never touches the filesystem; use
// EnsureDirs to materialize the directories when a write is actually needed.
func Resolve() Layout {
	return layoutFor(resolveHome())
}

func layoutFor(home string) Layout {
	return Layout{
		Home:         home,
		SystemConfig: resolveSystemConfig(),
		Lock:         filepath.Join(home, ".lock"),
		Config:       filepath.Join(home, configName),
		Schema:       filepath.Join(home, schemaName),
		Tools:        filepath.Join(home, toolsName),
		Logs:         filepath.Join(home, logsName),
		Cache:        filepath.Join(home, cacheName),
		Secrets:      filepath.Join(home, secretName),
	}
}

// resolveSystemConfig returns the machine-wide config path. $PROBACI_SYSTEM_DIR
// overrides; otherwise /etc/probaci on Unix and %ProgramData%\probaci on Windows.
func resolveSystemConfig() string {
	if v := os.Getenv("PROBACI_SYSTEM_DIR"); v != "" {
		return filepath.Join(v, configName)
	}
	if runtime.GOOS == "windows" {
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, appDir, configName)
	}
	return filepath.Join("/etc", appDir, configName)
}

func resolveHome() string {
	if v := os.Getenv(EnvHome); v != "" {
		return v
	}
	if v := os.Getenv(EnvXDGConfig); v != "" {
		return filepath.Join(v, appDir)
	}
	if runtime.GOOS == "windows" {
		if v := os.Getenv("AppData"); v != "" {
			return filepath.Join(v, appDir)
		}
	}
	// Linux & macOS default to the XDG-style ~/.config for a consistent layout.
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", appDir)
	}
	// Last resort: the OS-native config dir.
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, appDir)
	}
	return appDir
}

// EnsureDirs creates the home, logs, cache, and secrets directories if absent.
// The secrets directory is created 0700; everything else 0755.
func (l Layout) EnsureDirs() error {
	for _, d := range []string{l.Home, l.Logs, l.Cache} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return os.MkdirAll(l.Secrets, 0o700)
}

// CacheDir returns the OS cache dir for probaci, preferring os.UserCacheDir and
// falling back to the home-relative cache directory.
func CacheDir() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, appDir)
	}
	return Resolve().Cache
}
