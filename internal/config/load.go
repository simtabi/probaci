package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ProjectFileName is the per-repository config file.
const ProjectFileName = "probaci.json"

// Source labels where the effective config (or a layer of it) came from.
type Source string

const (
	SourceDefault Source = "built-in defaults"
	SourceSystem  Source = "system config"
	SourceUser    Source = "user config"
	SourceProject Source = "project config"
	SourceEnv     Source = "environment"
)

// LoadOptions controls the layered load.
type LoadOptions struct {
	// SystemConfigPath is the read-only machine-wide config (may be absent).
	SystemConfigPath string
	// UserConfigPath is the user-global config file (may be absent).
	UserConfigPath string
	// ProjectDir is the repository root; ./probaci.json is read from here.
	ProjectDir string
	// ExplicitPath, when set (e.g. via --config), replaces the project file.
	ExplicitPath string
}

// Load merges the configuration layers in precedence order and validates the
// result. Missing files are not errors; malformed files are.
func Load(opts LoadOptions) (Config, []Source, error) {
	cfg := Default()
	used := []Source{SourceDefault}

	if opts.SystemConfigPath != "" {
		layer, ok, err := readIfExists(opts.SystemConfigPath)
		if err != nil {
			return Config{}, nil, fmt.Errorf("system config %s: %w", opts.SystemConfigPath, err)
		}
		if ok {
			cfg = merge(cfg, layer)
			used = append(used, SourceSystem)
		}
	}

	if opts.UserConfigPath != "" {
		layer, ok, err := readIfExists(opts.UserConfigPath)
		if err != nil {
			return Config{}, nil, fmt.Errorf("user config %s: %w", opts.UserConfigPath, err)
		}
		if ok {
			cfg = merge(cfg, layer)
			used = append(used, SourceUser)
		}
	}

	projectPath := opts.ExplicitPath
	if projectPath == "" && opts.ProjectDir != "" {
		projectPath = filepath.Join(opts.ProjectDir, ProjectFileName)
	}
	if projectPath != "" {
		layer, ok, err := readIfExists(projectPath)
		if err != nil {
			return Config{}, nil, fmt.Errorf("project config %s: %w", projectPath, err)
		}
		if ok {
			cfg = merge(cfg, layer)
			used = append(used, SourceProject)
		}
	}

	if applyEnv(&cfg) {
		used = append(used, SourceEnv)
	}

	if err := Validate(cfg); err != nil {
		return Config{}, nil, err
	}
	return cfg, used, nil
}

func readIfExists(path string) (Config, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, false, nil
	}
	if err != nil {
		return Config{}, false, err
	}
	c, err := parse(data)
	if err != nil {
		return Config{}, false, err
	}
	return c, true, nil
}

// merge overlays src onto base, returning a new Config. Scalar fields override
// when set; stage lists replace wholesale (callers own ordering); maps merge
// per key.
func merge(base, src Config) Config {
	out := base.Clone()
	if src.Version != 0 {
		out.Version = src.Version
	}
	if len(src.Members) > 0 {
		out.Members = append([]string(nil), src.Members...)
	}
	mergeProject(&out.Project, src.Project)
	mergeDocker(&out.Docker, src.Docker)
	mergeSecurity(&out.Security, src.Security)
	if len(src.Stages) > 0 {
		out.Stages = append([]Stage(nil), src.Stages...)
	}
	for k, v := range src.Platforms {
		if out.Platforms == nil {
			out.Platforms = map[string]Platform{}
		}
		out.Platforms[k] = v
	}
	for k, v := range src.Tools {
		if out.Tools == nil {
			out.Tools = map[string]Tool{}
		}
		out.Tools[k] = v
	}
	if src.Secrets != "" {
		out.Secrets = src.Secrets
	}
	if src.EnvFile != "" {
		out.EnvFile = src.EnvFile
	}
	if src.TUI.Theme != "" {
		out.TUI.Theme = src.TUI.Theme
	}
	return out
}

func mergeProject(dst *Project, src Project) {
	if len(src.Languages) > 0 {
		dst.Languages = append([]string(nil), src.Languages...)
	}
	setStr(&dst.InstallCmd, src.InstallCmd)
	setStr(&dst.TestCmd, src.TestCmd)
	setStr(&dst.LintCmd, src.LintCmd)
	setStr(&dst.AuditCmd, src.AuditCmd)
}

func mergeDocker(dst *Docker, src Docker) {
	// Enabled and AllowSocketMount are booleans; treat the source as
	// authoritative only when the surrounding layer set non-zero markers.
	if src.Backend != "" {
		dst.Backend = src.Backend
	}
	if src.Pull != "" {
		dst.Pull = src.Pull
	}
	setStr(&dst.Network, src.Network)
	setStr(&dst.Resources.Memory, src.Resources.Memory)
	setStr(&dst.Resources.CPUs, src.Resources.CPUs)
	if src.Resources.PidsLimit != 0 {
		dst.Resources.PidsLimit = src.Resources.PidsLimit
	}
	// Tri-state: override only when the source layer explicitly set the value,
	// so an explicit `false` in any layer turns the feature off.
	if src.Enabled != nil {
		dst.Enabled = BoolPtr(*src.Enabled)
	}
	if src.AllowSocketMount != nil {
		dst.AllowSocketMount = BoolPtr(*src.AllowSocketMount)
	}
}

func mergeSecurity(dst *Security, src Security) {
	setStr(&dst.VerifyImages, src.VerifyImages)
	setStr(&dst.RegistryMirror, src.RegistryMirror)
	setStr(&dst.CosignIdentity, src.CosignIdentity)
	setStr(&dst.CosignIssuer, src.CosignIssuer)
	if len(src.AllowUnsigned) > 0 {
		dst.AllowUnsigned = append([]string(nil), src.AllowUnsigned...)
	}
}

func setStr(dst *string, src string) {
	if src != "" {
		*dst = src
	}
}

// applyEnv overlays PROBACI_* environment variables. Returns true if any were
// applied. Only a focused, documented set is supported.
func applyEnv(c *Config) bool {
	applied := false
	if v, ok := os.LookupEnv("PROBACI_BACKEND"); ok {
		c.Docker.Backend = Backend(v)
		applied = true
	}
	if v, ok := os.LookupEnv("PROBACI_PULL"); ok {
		c.Docker.Pull = PullPolicy(v)
		applied = true
	}
	if v, ok := os.LookupEnv("PROBACI_NO_DOCKER"); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			c.Docker.Enabled = BoolPtr(!b)
			applied = true
		}
	}
	if v, ok := os.LookupEnv("PROBACI_VERIFY_IMAGES"); ok {
		c.Security.VerifyImages = v
		applied = true
	}
	return applied
}

// Validate checks structural and value-level invariants.
func Validate(c Config) error {
	if c.Version != SchemaVersion {
		return fmt.Errorf("unsupported config version %d (want %d); run `probaci config migrate`", c.Version, SchemaVersion)
	}
	switch c.Docker.Backend {
	case BackendAuto, BackendDocker, BackendRootless, BackendPodman:
	default:
		return fmt.Errorf("invalid docker.backend %q (want auto|docker|rootless|podman)", c.Docker.Backend)
	}
	switch c.Docker.Pull {
	case PullMissing, PullAlways, PullNever:
	default:
		return fmt.Errorf("invalid docker.pull %q (want missing|always|never)", c.Docker.Pull)
	}
	switch c.Security.VerifyImages {
	case "advisory", "strict":
	default:
		return fmt.Errorf("invalid security.verify_images %q (want advisory|strict)", c.Security.VerifyImages)
	}
	known := map[string]bool{}
	for _, name := range DefaultStageOrder {
		known[name] = true
	}
	for _, s := range c.Stages {
		if strings.TrimSpace(s.Name) == "" {
			return errors.New("stage with empty name")
		}
		if !known[s.Name] {
			return fmt.Errorf("unknown stage %q (known: %s)", s.Name, strings.Join(DefaultStageOrder, ", "))
		}
	}
	return nil
}
