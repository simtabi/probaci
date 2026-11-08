// Package result defines the shared outcome types produced by stages and
// platform adapters. It is a leaf package so both the stage engine and the
// platform adapters can depend on it without an import cycle.
package result

import "time"

// Status is the outcome of a stage or a whole repository run.
type Status string

const (
	// StatusPass means the stage ran and succeeded.
	StatusPass Status = "pass"
	// StatusFail means the stage ran and failed.
	StatusFail Status = "fail"
	// StatusSkip means the stage was intentionally not run (missing tool,
	// nothing to do, disabled).
	StatusSkip Status = "skip"
	// StatusError means the stage could not run due to an operational problem
	// (e.g. Docker unavailable) rather than a check failure.
	StatusError Status = "error"
	// StatusPending means the stage has not run yet (used by the TUI).
	StatusPending Status = "pending"
	// StatusRunning means the stage is in progress (used by the TUI).
	StatusRunning Status = "running"
)

// OK reports whether the status should count as a passing outcome (pass or
// skip do not fail the pipeline).
func (s Status) OK() bool { return s == StatusPass || s == StatusSkip }

// Result is the outcome of a single stage for a single repository.
type Result struct {
	Stage    string        `json:"stage"`
	Status   Status        `json:"status"`
	Summary  string        `json:"summary,omitempty"`
	Output   string        `json:"output,omitempty"`
	Duration time.Duration `json:"duration_ns"`
	Err      string        `json:"error,omitempty"`
}

// RepoReport collects every stage result for one repository.
type RepoReport struct {
	Path    string   `json:"path"`
	Results []Result `json:"results"`
}

// Failed reports whether any stage in the repo failed or errored.
func (r RepoReport) Failed() bool {
	for _, res := range r.Results {
		if !res.Status.OK() {
			return true
		}
	}
	return false
}

// Aggregate is the top-level report across all repositories.
type Aggregate struct {
	Repos []RepoReport `json:"repos"`
}

// Failed reports whether any repository failed.
func (a Aggregate) Failed() bool {
	for _, r := range a.Repos {
		if r.Failed() {
			return true
		}
	}
	return false
}
