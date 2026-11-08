// Package stage holds the pipeline engine and the stage implementations. The
// engine runs each repository through its enabled stages, emitting structured
// events so the CLI and the TUI can share one runner.
package stage

import (
	"context"
	"sync"
	"time"

	"github.com/simtabi/probaci/internal/config"
	"github.com/simtabi/probaci/internal/detect"
	"github.com/simtabi/probaci/internal/docker"
	"github.com/simtabi/probaci/internal/platform"
	"github.com/simtabi/probaci/internal/result"
	"github.com/simtabi/probaci/internal/tool"
	"github.com/simtabi/probaci/internal/vcs"
)

// Event is emitted as the pipeline progresses.
type Event struct {
	Repo   string
	Stage  string
	Status result.Status
	Line   string         // a streaming output line (Status == StatusRunning)
	Result *result.Result // set when a stage finishes
}

// Observer receives events. It must be safe for concurrent use when Jobs > 1.
type Observer func(Event)

// Engine orchestrates stage execution.
type Engine struct {
	cfg    config.Config
	broker *docker.Broker // may be nil in --no-docker mode
	reg    *tool.Registry
	redact func(string) string
	// repoConfig, when set, resolves the config + tool registry for a specific
	// repository (its own probaci.json merged over system/user/defaults). When
	// nil, every repo uses the base cfg/reg.
	repoConfig func(dir string) (config.Config, *tool.Registry)
}

// SetRepoConfig installs a per-repository config resolver (see repoConfig).
func (e *Engine) SetRepoConfig(fn func(dir string) (config.Config, *tool.Registry)) {
	e.repoConfig = fn
}

// New constructs an Engine. broker may be nil; docker-dependent stages then skip.
func New(cfg config.Config, broker *docker.Broker, reg *tool.Registry, redact func(string) string) *Engine {
	if redact == nil {
		redact = func(s string) string { return s }
	}
	return &Engine{cfg: cfg, broker: broker, reg: reg, redact: redact}
}

// RunOptions controls a pipeline run.
type RunOptions struct {
	Repos    []string
	Only     []string
	Skip     []string
	Jobs     int
	Platform string // restrict to a single platform by name
	RunOpts  platform.RunOpts
}

// Run executes the pipeline across all repos and returns the aggregate report.
func (e *Engine) Run(ctx context.Context, opts RunOptions, obs Observer) result.Aggregate {
	if obs == nil {
		obs = func(Event) {}
	}
	jobs := opts.Jobs
	if jobs < 1 {
		jobs = 1
	}

	// Serialize observer calls: with Jobs > 1 the per-repo goroutines would
	// otherwise interleave on stdout / the TUI. One mutex keeps every emission
	// atomic without changing the Observer contract.
	var emitMu sync.Mutex
	safeObs := func(ev Event) {
		emitMu.Lock()
		obs(ev)
		emitMu.Unlock()
	}

	agg := result.Aggregate{Repos: make([]result.RepoReport, len(opts.Repos))}
	var mu sync.Mutex
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup

	for i, repo := range opts.Repos {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, repo string) {
			defer wg.Done()
			defer func() { <-sem }()
			report := e.runRepo(ctx, repo, opts, safeObs)
			mu.Lock()
			agg.Repos[i] = report
			mu.Unlock()
		}(i, repo)
	}
	wg.Wait()
	return agg
}

func (e *Engine) runRepo(ctx context.Context, repo string, opts RunOptions, obs Observer) result.RepoReport {
	report := result.RepoReport{Path: repo}

	plats := platform.Detect(repo)
	if opts.Platform != "" {
		plats = filterPlatform(plats, opts.Platform)
	}

	// Each repository uses its own merged config + tool registry when a resolver
	// is installed, so multi-repo runs honor per-repo probaci.json/tools.
	cfg, reg := e.cfg, e.reg
	if e.repoConfig != nil {
		cfg, reg = e.repoConfig(repo)
	}

	sc := &stageCtx{
		ctx:       ctx,
		repo:      repo,
		cfg:       cfg,
		broker:    e.broker,
		reg:       reg,
		project:   detect.DetectProject(repo),
		vcs:       vcs.Detect(repo),
		platforms: plats,
		runOpts:   opts.RunOpts,
		redact:    e.redact,
	}

	for _, name := range stageNames(cfg, opts.Only, opts.Skip) {
		if ctx.Err() != nil { // canceled (e.g. TUI quit) — stop promptly
			break
		}
		fn, ok := registry[name]
		if !ok {
			continue
		}
		obs(Event{Repo: repo, Stage: name, Status: result.StatusRunning})
		sc.emit = func(line string) {
			obs(Event{Repo: repo, Stage: name, Status: result.StatusRunning, Line: e.redact(line)})
		}
		start := time.Now()
		res := fn(sc)
		if res.Duration == 0 {
			res.Duration = time.Since(start)
		}
		res.Stage = name
		res.Output = e.redact(res.Output)
		res.Summary = e.redact(res.Summary)
		report.Results = append(report.Results, res)
		obs(Event{Repo: repo, Stage: name, Status: res.Status, Result: &res})
	}
	return report
}

// StageList resolves the ordered, filtered set of stage names for the given
// only/skip selections against the base config (used by `run --dry-run`).
func (e *Engine) StageList(only, skip []string) []string {
	return stageNames(e.cfg, only, skip)
}

// stageNames resolves the ordered, filtered set of stage names for a config.
func stageNames(cfg config.Config, only, skip []string) []string {
	base := cfg.EnabledStages()
	if len(only) > 0 {
		base = only
	}
	skipSet := map[string]bool{}
	for _, s := range skip {
		skipSet[s] = true
	}
	out := make([]string, 0, len(base))
	for _, s := range base {
		if !skipSet[s] {
			out = append(out, s)
		}
	}
	return out
}

func filterPlatform(ps []platform.Platform, name string) []platform.Platform {
	var out []platform.Platform
	for _, p := range ps {
		if p.Name() == name {
			out = append(out, p)
		}
	}
	return out
}
