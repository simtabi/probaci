// Package probaci is the public, embeddable API: it assembles the config,
// container broker, tool registry, secret redactor, logger, and stage engine
// into a ready-to-run App. Other Go programs can import this to drive probaci
// without the CLI.
package probaci

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/simtabi/probaci/internal/config"
	"github.com/simtabi/probaci/internal/docker"
	"github.com/simtabi/probaci/internal/logging"
	"github.com/simtabi/probaci/internal/paths"
	"github.com/simtabi/probaci/internal/run"
	"github.com/simtabi/probaci/internal/secret"
	"github.com/simtabi/probaci/internal/stage"
	"github.com/simtabi/probaci/internal/tool"
)

// App is a fully-wired probaci instance.
type App struct {
	Cfg      config.Config
	Sources  []config.Source
	Paths    paths.Layout
	RunID    run.ID
	Broker   *docker.Broker // nil when docker is disabled/unavailable
	Registry *tool.Registry
	Engine   *stage.Engine
	Redactor *secret.Redactor
	Logger   *slog.Logger

	closer io.Closer
}

// Options configures App construction.
type Options struct {
	// ProjectDir is the repository root used to find ./probaci.json (and as the
	// default run target).
	ProjectDir string
	// ConfigPath, when set, overrides the project config file (--config).
	ConfigPath string
	// NoDocker disables the broker even if a runtime is present.
	NoDocker bool
	// Backend / Pull override config values when non-empty.
	Backend string
	Pull    string
	// LogLevel sets the file-log verbosity.
	LogLevel slog.Level
	// Command names the subcommand, used in the per-run log filename.
	Command string
}

// New loads configuration and assembles the App. A missing container runtime is
// not fatal here; docker-dependent stages skip with guidance at run time.
func New(opts Options) (*App, error) {
	layout := paths.Resolve()
	_ = layout.EnsureDirs() // best effort; tool still works without a home

	cfg, sources, err := config.Load(config.LoadOptions{
		SystemConfigPath: layout.SystemConfig,
		UserConfigPath:   layout.Config,
		ProjectDir:       opts.ProjectDir,
		ExplicitPath:     opts.ConfigPath,
	})
	if err != nil {
		return nil, err
	}
	applyOverrides(&cfg, opts)

	runID := run.New(time.Now())

	red := secret.New()
	red.Add(loadSecretValues(cfg.Secrets, opts.ProjectDir)...)

	logger, closer := logging.New(logging.Options{
		Dir:      layout.Logs,
		Level:    opts.LogLevel,
		RunID:    runID.Short,
		Command:  opts.Command,
		Repo:     filepath.Base(opts.ProjectDir),
		Now:      runID.Started,
		Redactor: red.Func(),
	})

	app := &App{
		Cfg:      cfg,
		Sources:  sources,
		Paths:    layout,
		RunID:    runID,
		Registry: tool.New(cfg.Tools),
		Redactor: red,
		Logger:   logger,
		closer:   closer,
	}

	if cfg.Docker.IsEnabled() {
		labels := map[string]string{
			docker.LabelMarker: "true",
			docker.LabelUser:   currentUser(),
			docker.LabelRun:    runID.Short,
		}
		broker, err := docker.New(cfg.Docker, cfg.Security, labels)
		if err != nil {
			// Record but don't fail; stages that need docker will skip.
			logger.Warn("container runtime unavailable", "error", err.Error())
		} else {
			app.Broker = broker
		}
	}

	app.Engine = stage.New(cfg, app.Broker, app.Registry, red.Func())

	// Per-repo config: each repository in a multi-repo run uses its own
	// probaci.json (merged over system/user/defaults) and its own tool registry.
	// An explicit --config applies to every repo; otherwise each repo's local
	// file is read. Falls back to the base config on any load error.
	app.Engine.SetRepoConfig(func(dir string) (config.Config, *tool.Registry) {
		rc, _, err := config.Load(config.LoadOptions{
			SystemConfigPath: layout.SystemConfig,
			UserConfigPath:   layout.Config,
			ProjectDir:       dir,
			ExplicitPath:     opts.ConfigPath,
		})
		if err != nil {
			return cfg, app.Registry
		}
		applyOverrides(&rc, opts)
		return rc, tool.New(rc.Tools)
	})
	return app, nil
}

// applyOverrides applies CLI/option overrides (no-docker, backend, pull) on top
// of a loaded config.
func applyOverrides(cfg *config.Config, opts Options) {
	if opts.NoDocker {
		cfg.Docker.Enabled = config.BoolPtr(false)
	}
	if opts.Backend != "" {
		cfg.Docker.Backend = config.Backend(opts.Backend)
	}
	if opts.Pull != "" {
		cfg.Docker.Pull = config.PullPolicy(opts.Pull)
	}
}

// CheckBroker verifies the container runtime is reachable, returning a non-nil
// error with remediation guidance when it is not.
func (a *App) CheckBroker(ctx context.Context) error {
	if a.Broker == nil {
		return nil
	}
	return a.Broker.Available(ctx)
}

// Close flushes and closes the logger.
func (a *App) Close() error {
	if a.closer != nil {
		return a.closer.Close()
	}
	return nil
}

// loadSecretValues reads KEY=value lines from the project's secrets file (if
// present) so their values can be registered with the redactor.
func loadSecretValues(secretsFile, projectDir string) []string {
	if secretsFile == "" {
		return nil
	}
	path := secretsFile
	if !isAbs(path) && projectDir != "" {
		path = projectDir + string(os.PathSeparator) + secretsFile
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var vals []string
	for _, line := range splitLines(string(data)) {
		if i := indexByte(line, '='); i > 0 {
			vals = append(vals, line[i+1:])
		}
	}
	return vals
}

// currentUser returns a stable label value for the running user.
func currentUser() string {
	if u, err := user.Current(); err == nil {
		if u.Username != "" {
			return u.Username
		}
		return u.Uid
	}
	return "unknown"
}

func isAbs(p string) bool { return len(p) > 0 && (p[0] == '/' || (len(p) > 2 && p[1] == ':')) }

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, trimCR(s[start:i]))
			start = i + 1
		}
	}
	out = append(out, trimCR(s[start:]))
	return out
}

func trimCR(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\r' {
		return s[:len(s)-1]
	}
	return s
}
