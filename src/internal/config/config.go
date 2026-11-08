// Package config defines the probaci.json schema and the layered loader that
// merges built-in defaults, the user-global config, the per-project config,
// environment variables, and CLI flags.
//
// Precedence (lowest to highest):
//
//	embedded defaults  ->  ~/.config/probaci/config.json  ->  ./probaci.json
//	->  PROBACI_* env  ->  CLI flags
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SchemaVersion is the current config schema version.
const SchemaVersion = 1

// Config is the root probaci configuration document.
type Config struct {
	Version   int                 `json:"version"`
	Members   []string            `json:"members,omitempty"`
	Project   Project             `json:"project"`
	Docker    Docker              `json:"docker"`
	Security  Security            `json:"security"`
	Platforms map[string]Platform `json:"platforms,omitempty"`
	Stages    []Stage             `json:"stages"`
	Tools     map[string]Tool     `json:"tools,omitempty"`
	Secrets   string              `json:"secrets_file,omitempty"`
	EnvFile   string              `json:"env_file,omitempty"`
	TUI       TUI                 `json:"tui"`
}

// Project carries language and command overrides; empty fields are
// auto-detected from the repository.
type Project struct {
	Languages  []string `json:"languages,omitempty"`
	InstallCmd string   `json:"install_cmd,omitempty"`
	TestCmd    string   `json:"test_cmd,omitempty"`
	LintCmd    string   `json:"lint_cmd,omitempty"`
	AuditCmd   string   `json:"audit_cmd,omitempty"`
}

// Backend selects the container runtime used by the broker.
type Backend string

const (
	BackendAuto     Backend = "auto"
	BackendDocker   Backend = "docker"
	BackendRootless Backend = "rootless"
	BackendPodman   Backend = "podman"
)

// PullPolicy controls when images are pulled.
type PullPolicy string

const (
	PullMissing PullPolicy = "missing"
	PullAlways  PullPolicy = "always"
	PullNever   PullPolicy = "never"
)

// Docker configures the container broker. Enabled and AllowSocketMount are
// tri-state (*bool) so an absent value inherits the lower layer while an
// explicit false in any layer can turn the feature off — a plain bool can't
// distinguish "unset" from "false" during merge.
type Docker struct {
	Enabled   *bool      `json:"enabled,omitempty"`
	Backend   Backend    `json:"backend"`
	Pull      PullPolicy `json:"pull"`
	Network   string     `json:"network,omitempty"`
	Resources Resources  `json:"resources"`
	// AllowSocketMount opts in to mounting the container socket for nested-Docker
	// workflows (act, gitlab-ci-local). Off by default; see docs/security.md.
	AllowSocketMount *bool `json:"allow_socket_mount,omitempty"`
}

// IsEnabled reports whether the broker is enabled (default true when unset).
func (d Docker) IsEnabled() bool { return d.Enabled == nil || *d.Enabled }

// SocketMountAllowed reports whether the container socket may be mounted
// (default false when unset).
func (d Docker) SocketMountAllowed() bool { return d.AllowSocketMount != nil && *d.AllowSocketMount }

// BoolPtr returns a pointer to b (helper for building configs).
func BoolPtr(b bool) *bool { return &b }

// Resources caps container resource usage.
type Resources struct {
	Memory    string `json:"memory,omitempty"` // e.g. "2g"
	CPUs      string `json:"cpus,omitempty"`   // e.g. "2"
	PidsLimit int    `json:"pids_limit,omitempty"`
}

// Security holds image-trust and verification policy.
type Security struct {
	// VerifyImages: "advisory" (verify+warn, default) or "strict" (block).
	VerifyImages string `json:"verify_images"`
	// AllowUnsigned is the allow-list of image references permitted under strict
	// mode even when unpinned/unverified (prefix match on the image, no tag/digest).
	AllowUnsigned []string `json:"allow_unsigned,omitempty"`
	// RegistryMirror optionally rewrites image hosts to a private mirror.
	RegistryMirror string `json:"registry_mirror,omitempty"`
	// CosignIdentity / CosignIssuer enable keyless cosign verification when set
	// (and the `cosign` binary is on PATH). Left blank, verification is skipped.
	CosignIdentity string `json:"cosign_identity,omitempty"`
	CosignIssuer   string `json:"cosign_issuer,omitempty"`
}

// Platform is per-CI-service config (endpoints + token env names).
type Platform struct {
	Enabled  bool   `json:"enabled"`
	BaseURL  string `json:"base_url,omitempty"`
	TokenEnv string `json:"token_env,omitempty"`
}

// Stage is one entry in the ordered pipeline.
type Stage struct {
	Name    string         `json:"name"`
	Enabled bool           `json:"enabled"`
	Options map[string]any `json:"options,omitempty"`
}

// Tool is a registry entry or override.
type Tool struct {
	Image       string   `json:"image,omitempty"`
	Tag         string   `json:"tag,omitempty"`
	Digest      string   `json:"digest,omitempty"`
	Entrypoint  string   `json:"entrypoint,omitempty"`
	CmdTemplate []string `json:"cmd,omitempty"`
	Exec        string   `json:"exec,omitempty"` // docker|local|auto
	Languages   []string `json:"languages,omitempty"`
}

// TUI holds dashboard preferences.
type TUI struct {
	Theme string `json:"theme"` // auto|dark|light
}

// Clone returns a deep copy of the config so merges never alias shared maps.
func (c Config) Clone() Config {
	out := c
	out.Members = append([]string(nil), c.Members...)
	out.Project.Languages = append([]string(nil), c.Project.Languages...)
	out.Stages = append([]Stage(nil), c.Stages...)
	if c.Platforms != nil {
		out.Platforms = make(map[string]Platform, len(c.Platforms))
		for k, v := range c.Platforms {
			out.Platforms[k] = v
		}
	}
	if c.Tools != nil {
		out.Tools = make(map[string]Tool, len(c.Tools))
		for k, v := range c.Tools {
			out.Tools[k] = v
		}
	}
	out.Security.AllowUnsigned = append([]string(nil), c.Security.AllowUnsigned...)
	if c.Docker.Enabled != nil {
		out.Docker.Enabled = BoolPtr(*c.Docker.Enabled)
	}
	if c.Docker.AllowSocketMount != nil {
		out.Docker.AllowSocketMount = BoolPtr(*c.Docker.AllowSocketMount)
	}
	return out
}

// Parse decodes JSON into a Config, rejecting unknown fields so typos surface
// as errors rather than being silently ignored.
func Parse(data []byte) (Config, error) { return parse(data) }

// parse decodes JSON into a Config, rejecting unknown fields so typos surface
// as errors rather than silently ignored keys.
func parse(data []byte) (Config, error) {
	var c Config
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return c, nil
}

// writeJSON marshals a config to a file with stable, indented formatting,
// writing atomically (temp file in the same directory, then rename) so a
// concurrent reader never observes a half-written file.
func writeJSON(path string, c Config, perm os.FileMode) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return atomicWrite(path, data, perm)
}

// atomicWrite writes data to path via a temp file + rename on the same
// filesystem.
func atomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".probaci-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op if the rename succeeded
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
