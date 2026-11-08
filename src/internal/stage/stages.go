package stage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/simtabi/probaci/internal/config"
	"github.com/simtabi/probaci/internal/detect"
	"github.com/simtabi/probaci/internal/docker"
	"github.com/simtabi/probaci/internal/platform"
	"github.com/simtabi/probaci/internal/result"
	"github.com/simtabi/probaci/internal/tool"
	"github.com/simtabi/probaci/internal/vcs"
)

// stageCtx carries everything a stage needs for one repository.
type stageCtx struct {
	ctx       context.Context
	repo      string
	cfg       config.Config
	broker    *docker.Broker
	reg       *tool.Registry
	project   detect.Project
	vcs       vcs.VCS
	platforms []platform.Platform
	runOpts   platform.RunOpts
	emit      func(string)
	redact    func(string) string
}

// StageFunc runs one stage and returns its result.
type StageFunc func(c *stageCtx) result.Result

// registry maps stage names to implementations.
var registry = map[string]StageFunc{
	"detect":          stageDetect,
	"workflow-lint":   stageWorkflowLint,
	"secrets":         stageSecrets,
	"versions":        stageVersions,
	"lint":            stageLint,
	"clean-clone":     stageCleanClone,
	"audit":           stageAudit,
	"sast":            stageSAST,
	"dockerfile-lint": stageDockerfileLint,
	"yaml-lint":       stageYAMLLint,
	"container-scan":  stageContainerScan,
	"commitlint":      stageCommitlint,
	"workflow-run":    stageWorkflowRun,
}

func skip(msg string) result.Result {
	return result.Result{Status: result.StatusSkip, Summary: msg}
}
func pass(msg, out string) result.Result {
	return result.Result{Status: result.StatusPass, Summary: msg, Output: out}
}
func failr(msg, out string) result.Result {
	return result.Result{Status: result.StatusFail, Summary: msg, Output: out}
}

func (c *stageCtx) deps() platform.Deps {
	return platform.Deps{Broker: c.broker, Registry: c.reg, Emit: c.emit}
}

// brokerTool runs a registry tool over the repo and maps exit status to result.
func (c *stageCtx) brokerTool(toolName string, extra ...string) result.Result {
	if c.broker == nil {
		return skip("no container runtime; " + toolName + " skipped")
	}
	t, err := c.reg.Resolve(toolName)
	if err != nil {
		return skip(err.Error())
	}
	args := t.Args
	if len(extra) > 0 {
		args = append(append([]string(nil), t.Args...), extra...)
	}
	network := ""
	if t.NeedsNet {
		network = "bridge"
	}
	out, runErr := c.broker.Capture(c.ctx, docker.RunSpec{
		Image:      t.Ref(),
		Entrypoint: t.Entrypoint,
		Args:       args,
		RepoMount:  c.repo,
		Network:    network,
	})
	if c.emit != nil && out != "" {
		c.emit(strings.TrimRight(out, "\n"))
	}
	if runErr != nil {
		var blocked *docker.ImageBlockedError
		if errors.As(runErr, &blocked) {
			return result.Result{Status: result.StatusError, Summary: blocked.Error()}
		}
		return failr(toolName+" reported problems", out)
	}
	return pass(toolName+" clean", out)
}

func stageDetect(c *stageCtx) result.Result {
	var langs []string
	for _, l := range c.project.Languages {
		langs = append(langs, l.Name)
	}
	var plats []string
	for _, p := range c.platforms {
		plats = append(plats, fmt.Sprintf("%s(%s)", p.Name(), p.Tier()))
	}
	vcsName := "none"
	if c.vcs != nil {
		vcsName = c.vcs.Name()
	}
	summary := fmt.Sprintf("vcs=%s languages=[%s] platforms=[%s]",
		vcsName, strings.Join(langs, ","), strings.Join(plats, ","))
	if c.project.NoLock {
		summary += " (no lockfile — CI installs may drift)"
	}
	return pass(summary, "")
}

func stageWorkflowLint(c *stageCtx) result.Result {
	if len(c.platforms) == 0 {
		return skip("no CI workflow files detected")
	}
	var outputs []string
	worst := result.StatusSkip
	for _, p := range c.platforms {
		r := p.Lint(c.ctx, c.deps(), c.repo)
		outputs = append(outputs, fmt.Sprintf("[%s] %s", p.Name(), r.Summary))
		worst = worsen(worst, r.Status)
	}
	return result.Result{Status: worst, Summary: "linted " + plural(len(c.platforms), "platform"), Output: strings.Join(outputs, "\n")}
}

func stageSecrets(c *stageCtx) result.Result {
	return c.brokerTool("gitleaks")
}

func stageVersions(c *stageCtx) result.Result {
	// Advisory only: parse pinned runtime versions from workflow files and note
	// them. Never fails the pipeline (matches the original tool's behavior).
	var notes []string
	for _, p := range c.platforms {
		for _, f := range p.WorkflowFiles(c.repo) {
			data, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			for _, key := range []string{"node-version", "python-version", "go-version"} {
				if v := grepVersion(string(data), key); v != "" {
					notes = append(notes, fmt.Sprintf("%s pins %s=%s", filepath.Base(f), key, v))
				}
			}
		}
	}
	if len(notes) == 0 {
		return skip("no pinned runtime versions found to compare")
	}
	return pass("advisory: "+strings.Join(notes, "; "), "")
}

func stageLint(c *stageCtx) result.Result {
	if len(c.project.Languages) == 0 {
		return skip("no language detected to lint")
	}
	return c.perLanguage(func(l detect.Language) string { return l.LintTool })
}

func stageAudit(c *stageCtx) result.Result {
	if len(c.project.Languages) == 0 {
		return skip("no language detected to audit")
	}
	return c.perLanguage(func(l detect.Language) string { return l.AuditTool })
}

// perLanguage runs the tool named by pick for each detected language and folds
// the results.
func (c *stageCtx) perLanguage(pick func(detect.Language) string) result.Result {
	var outputs []string
	worst := result.StatusSkip
	ran := 0
	for _, l := range c.project.Languages {
		name := pick(l)
		if name == "" {
			continue
		}
		r := c.brokerTool(name)
		ran++
		outputs = append(outputs, fmt.Sprintf("[%s/%s] %s", l.Name, name, r.Summary))
		worst = worsen(worst, r.Status)
	}
	if ran == 0 {
		return skip("no applicable tool for detected languages")
	}
	return result.Result{Status: worst, Summary: plural(ran, "tool") + " run", Output: strings.Join(outputs, "\n")}
}

func stageCleanClone(c *stageCtx) result.Result {
	if c.vcs == nil {
		return skip("no VCS detected; cannot reproduce committed state")
	}
	dirty, err := c.vcs.IsDirty(c.ctx, c.repo)
	if err == nil && dirty && c.emit != nil {
		c.emit("warning: uncommitted/untracked files exist; CI will NOT see them")
	}
	test := c.testCmd()
	if test == "" {
		return skip("no test command configured/detected")
	}
	dest, err := os.MkdirTemp("", "probaci-clean-")
	if err != nil {
		return result.Result{Status: result.StatusError, Summary: "tempdir", Err: err.Error()}
	}
	defer os.RemoveAll(dest)
	exportInto := filepath.Join(dest, "repo")
	if err := c.vcs.ExportCommitted(c.ctx, c.repo, exportInto); err != nil {
		return result.Result{Status: result.StatusError, Summary: "export committed state", Err: err.Error()}
	}
	if c.broker == nil {
		return skip("no container runtime; clean-clone needs a runtime to install+test")
	}
	install := c.installCmd()
	script := test
	if install != "" {
		script = install + " && " + test
	}
	out, runErr := c.broker.Capture(c.ctx, docker.RunSpec{
		Image:      c.cleanCloneImage(),
		Entrypoint: "/bin/sh",
		Args:       []string{"-c", script},
		RepoMount:  exportInto,
		Writable:   true,
		Network:    "bridge", // installs need the network
		// The container runs as a non-root host UID with no writable home, so
		// point tool caches at the writable workspace/tmp to avoid EACCES.
		Env: map[string]string{
			"HOME":             "/tmp",
			"XDG_CACHE_HOME":   "/tmp/.cache",
			"GOCACHE":          "/tmp/.gocache",
			"NPM_CONFIG_CACHE": "/tmp/.npm",
		},
	})
	if c.emit != nil && out != "" {
		c.emit(strings.TrimRight(out, "\n"))
	}
	if runErr != nil {
		return failr("clean clone failed — this is exactly what CI sees (uncommitted file or missing lockfile dep)", out)
	}
	return pass("clean clone installed + tested OK", out)
}

func stageSAST(c *stageCtx) result.Result          { return c.brokerTool("semgrep") }
func stageYAMLLint(c *stageCtx) result.Result      { return c.brokerTool("yamllint") }
func stageContainerScan(c *stageCtx) result.Result { return c.brokerTool("trivy") }

func stageDockerfileLint(c *stageCtx) result.Result {
	if !fileExists(c.repo, "Dockerfile") {
		return skip("no Dockerfile present")
	}
	return c.brokerTool("hadolint", "Dockerfile")
}

func stageCommitlint(c *stageCtx) result.Result {
	if c.vcs == nil || c.vcs.Name() != "git" {
		return skip("commitlint needs a git repository")
	}
	return c.brokerTool("commitlint", "--from", "HEAD~1", "--to", "HEAD")
}

func stageWorkflowRun(c *stageCtx) result.Result {
	if len(c.platforms) == 0 {
		return skip("no CI workflow files detected")
	}
	var outputs []string
	worst := result.StatusSkip
	for _, p := range c.platforms {
		r := p.RunWorkflows(c.ctx, c.deps(), c.repo, c.runOpts)
		outputs = append(outputs, fmt.Sprintf("[%s] %s", p.Name(), r.Summary))
		worst = worsen(worst, r.Status)
	}
	return result.Result{Status: worst, Summary: "ran " + plural(len(c.platforms), "platform"), Output: strings.Join(outputs, "\n")}
}
