// Package platform contains one adapter per CI/VCS service. Adapters detect a
// service's workflow files, lint them, and (for Tier 1 services) run the real
// pipeline locally. This layer is orthogonal to the version-control layer in
// package vcs.
package platform

import (
	"context"

	"github.com/simtabi/probaci/internal/docker"
	"github.com/simtabi/probaci/internal/result"
	"github.com/simtabi/probaci/internal/tool"
)

// Tier expresses how much an adapter can honestly do.
type Tier int

const (
	// TierRun: lint + run the pipeline locally.
	TierRun Tier = 1
	// TierLint: lint/validate only.
	TierLint Tier = 2
	// TierDetect: detect + report only.
	TierDetect Tier = 3
)

func (t Tier) String() string {
	switch t {
	case TierRun:
		return "lint+run"
	case TierLint:
		return "lint"
	default:
		return "detect"
	}
}

// Deps are the shared dependencies passed to adapter operations.
type Deps struct {
	Broker   *docker.Broker
	Registry *tool.Registry
	// Emit streams a human-readable line to the observer (TUI/CLI).
	Emit func(string)
}

// RunOpts tunes a workflow run.
type RunOpts struct {
	Event            string
	Job              string
	FullImage        bool
	AllowSocketMount bool
}

// Platform is a CI service adapter.
type Platform interface {
	Name() string
	Tier() Tier
	Detect(repo string) bool
	WorkflowFiles(repo string) []string
	Lint(ctx context.Context, d Deps, repo string) result.Result
	RunWorkflows(ctx context.Context, d Deps, repo string, opts RunOpts) result.Result
}

// All returns every registered adapter in detection order.
func All() []Platform {
	return []Platform{
		github{},
		gitlab{},
		gitea{},
		bitbucket{},
		circleci{},
		drone{},
		woodpecker{},
		azure{},
		jenkins{},
		travis{},
		buildkite{},
	}
}

// Detect returns every adapter whose workflow files are present in repo.
func Detect(repo string) []Platform {
	var found []Platform
	for _, p := range All() {
		if p.Detect(repo) {
			found = append(found, p)
		}
	}
	return found
}

// ByName returns the adapter with the given name, or nil.
func ByName(name string) Platform {
	for _, p := range All() {
		if p.Name() == name {
			return p
		}
	}
	return nil
}
