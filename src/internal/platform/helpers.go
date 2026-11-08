package platform

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/simtabi/probaci/internal/docker"
	"github.com/simtabi/probaci/internal/result"
)

// fileExists reports whether repo/rel exists.
func fileExists(repo, rel string) bool {
	_, err := os.Stat(filepath.Join(repo, rel))
	return err == nil
}

// glob returns workflow files matching any of the relative glob patterns.
func glob(repo string, patterns ...string) []string {
	var out []string
	for _, p := range patterns {
		matches, _ := filepath.Glob(filepath.Join(repo, p))
		out = append(out, matches...)
	}
	return out
}

func skip(stage, msg string) result.Result {
	return result.Result{Stage: stage, Status: result.StatusSkip, Summary: msg}
}

func pass(stage, msg, output string) result.Result {
	return result.Result{Stage: stage, Status: result.StatusPass, Summary: msg, Output: output}
}

func fail(stage, msg, output string) result.Result {
	return result.Result{Stage: stage, Status: result.StatusFail, Summary: msg, Output: output}
}

// brokerTool runs a registry tool in a container over the repo and turns its
// exit status into a Result.
func brokerTool(ctx context.Context, d Deps, repo, stage, toolName string, args ...string) result.Result {
	if d.Broker == nil {
		return skip(stage, "no container runtime; "+toolName+" skipped")
	}
	t, err := d.Registry.Resolve(toolName)
	if err != nil {
		return skip(stage, err.Error())
	}
	cmdArgs := t.Args
	if len(args) > 0 {
		cmdArgs = append(append([]string(nil), t.Args...), args...)
	}
	network := ""
	if t.NeedsNet {
		network = "bridge"
	}
	out, runErr := d.Broker.Capture(ctx, docker.RunSpec{
		Image:      t.Ref(),
		Entrypoint: t.Entrypoint,
		Args:       cmdArgs,
		RepoMount:  repo,
		Network:    network,
	})
	if d.Emit != nil && out != "" {
		d.Emit(strings.TrimRight(out, "\n"))
	}
	if runErr != nil {
		var blocked *docker.ImageBlockedError
		if errors.As(runErr, &blocked) {
			return result.Result{Stage: stage, Status: result.StatusError, Summary: blocked.Error()}
		}
		return fail(stage, toolName+" reported problems", out)
	}
	return pass(stage, toolName+" clean", out)
}

// hostTool runs a host CLI (e.g. act, gitlab-ci-local) when present, streaming
// output. Used for runners that must drive the host container daemon.
func hostTool(ctx context.Context, d Deps, repo, stage, bin string, args ...string) result.Result {
	if _, err := exec.LookPath(bin); err != nil {
		return skip(stage, bin+" not installed — install it to enable this stage")
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = repo
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	start := time.Now()
	runErr := cmd.Run()
	out := buf.String()
	if d.Emit != nil && out != "" {
		d.Emit(strings.TrimRight(out, "\n"))
	}
	r := pass(stage, bin+" passed", out)
	if runErr != nil {
		r = fail(stage, bin+" failed", out)
	}
	r.Duration = time.Since(start)
	return r
}
