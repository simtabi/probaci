package platform

import (
	"context"

	"github.com/simtabi/probaci/internal/result"
)

// --- GitHub Actions (Tier 1) ----------------------------------------------

type github struct{}

func (github) Name() string { return "github" }
func (github) Tier() Tier   { return TierRun }
func (github) Detect(r string) bool {
	return len(glob(r, ".github/workflows/*.yml", ".github/workflows/*.yaml")) > 0
}
func (github) WorkflowFiles(r string) []string {
	return glob(r, ".github/workflows/*.yml", ".github/workflows/*.yaml")
}
func (github) Lint(ctx context.Context, d Deps, r string) result.Result {
	return brokerTool(ctx, d, r, "workflow-lint", "actionlint")
}
func (github) RunWorkflows(ctx context.Context, d Deps, r string, o RunOpts) result.Result {
	event := o.Event
	if event == "" {
		event = "push"
	}
	args := []string{event}
	if o.Job != "" {
		args = append(args, "-j", o.Job)
	}
	if o.FullImage {
		args = append(args, "-P", "ubuntu-latest=catthehacker/ubuntu:full-latest")
	}
	return hostTool(ctx, d, r, "workflow-run", "act", args...)
}

// --- GitLab CI (Tier 1) ----------------------------------------------------

type gitlab struct{}

func (gitlab) Name() string         { return "gitlab" }
func (gitlab) Tier() Tier           { return TierRun }
func (gitlab) Detect(r string) bool { return fileExists(r, ".gitlab-ci.yml") }
func (gitlab) WorkflowFiles(r string) []string {
	return glob(r, ".gitlab-ci.yml")
}
func (gitlab) Lint(ctx context.Context, d Deps, r string) result.Result {
	if res := hostTool(ctx, d, r, "workflow-lint", "gitlab-ci-local", "--preview"); res.Status != result.StatusSkip {
		return res
	}
	return brokerTool(ctx, d, r, "workflow-lint", "yamllint", ".gitlab-ci.yml")
}
func (gitlab) RunWorkflows(ctx context.Context, d Deps, r string, o RunOpts) result.Result {
	return hostTool(ctx, d, r, "workflow-run", "gitlab-ci-local")
}

// --- Gitea / Forgejo Actions (Tier 1, act-compatible) ----------------------

type gitea struct{}

func (gitea) Name() string { return "gitea" }
func (gitea) Tier() Tier   { return TierRun }
func (gitea) Detect(r string) bool {
	return len(glob(r, ".gitea/workflows/*.yml", ".gitea/workflows/*.yaml",
		".forgejo/workflows/*.yml", ".forgejo/workflows/*.yaml")) > 0
}
func (gitea) WorkflowFiles(r string) []string {
	return glob(r, ".gitea/workflows/*.yml", ".gitea/workflows/*.yaml",
		".forgejo/workflows/*.yml", ".forgejo/workflows/*.yaml")
}
func (gitea) Lint(ctx context.Context, d Deps, r string) result.Result {
	return brokerTool(ctx, d, r, "workflow-lint", "actionlint")
}
func (gitea) RunWorkflows(ctx context.Context, d Deps, r string, o RunOpts) result.Result {
	return hostTool(ctx, d, r, "workflow-run", "act", "-W", ".gitea/workflows")
}

// --- Bitbucket Pipelines (Tier 1) ------------------------------------------

type bitbucket struct{}

func (bitbucket) Name() string { return "bitbucket" }

// Tier is lint-only: local per-step container execution isn't implemented yet,
// so RunWorkflows skips with a note (kept honest via the platform tiering).
func (bitbucket) Tier() Tier           { return TierLint }
func (bitbucket) Detect(r string) bool { return fileExists(r, "bitbucket-pipelines.yml") }
func (bitbucket) WorkflowFiles(r string) []string {
	return glob(r, "bitbucket-pipelines.yml")
}
func (bitbucket) Lint(ctx context.Context, d Deps, r string) result.Result {
	return brokerTool(ctx, d, r, "workflow-lint", "yamllint", "bitbucket-pipelines.yml")
}
func (bitbucket) RunWorkflows(ctx context.Context, d Deps, r string, o RunOpts) result.Result {
	return skip("workflow-run", "bitbucket: per-step container runs are planned; lint covered")
}

// --- CircleCI (Tier 1) -----------------------------------------------------

type circleci struct{}

func (circleci) Name() string         { return "circleci" }
func (circleci) Tier() Tier           { return TierRun }
func (circleci) Detect(r string) bool { return fileExists(r, ".circleci/config.yml") }
func (circleci) WorkflowFiles(r string) []string {
	return glob(r, ".circleci/config.yml")
}
func (circleci) Lint(ctx context.Context, d Deps, r string) result.Result {
	return hostTool(ctx, d, r, "workflow-lint", "circleci", "config", "validate")
}
func (circleci) RunWorkflows(ctx context.Context, d Deps, r string, o RunOpts) result.Result {
	return hostTool(ctx, d, r, "workflow-run", "circleci", "local", "execute")
}

// --- Drone (Tier 1) --------------------------------------------------------

type drone struct{}

func (drone) Name() string                    { return "drone" }
func (drone) Tier() Tier                      { return TierRun }
func (drone) Detect(r string) bool            { return fileExists(r, ".drone.yml") }
func (drone) WorkflowFiles(r string) []string { return glob(r, ".drone.yml") }
func (drone) Lint(ctx context.Context, d Deps, r string) result.Result {
	return hostTool(ctx, d, r, "workflow-lint", "drone", "lint")
}
func (drone) RunWorkflows(ctx context.Context, d Deps, r string, o RunOpts) result.Result {
	return hostTool(ctx, d, r, "workflow-run", "drone", "exec")
}

// --- Woodpecker (Tier 1) ---------------------------------------------------

type woodpecker struct{}

func (woodpecker) Name() string { return "woodpecker" }
func (woodpecker) Tier() Tier   { return TierRun }
func (woodpecker) Detect(r string) bool {
	return fileExists(r, ".woodpecker.yml") || len(glob(r, ".woodpecker/*.yml", ".woodpecker/*.yaml")) > 0
}
func (woodpecker) WorkflowFiles(r string) []string {
	return append(glob(r, ".woodpecker.yml"), glob(r, ".woodpecker/*.yml", ".woodpecker/*.yaml")...)
}
func (woodpecker) Lint(ctx context.Context, d Deps, r string) result.Result {
	return hostTool(ctx, d, r, "workflow-lint", "woodpecker-cli", "lint")
}
func (woodpecker) RunWorkflows(ctx context.Context, d Deps, r string, o RunOpts) result.Result {
	return hostTool(ctx, d, r, "workflow-run", "woodpecker-cli", "exec")
}

// --- Azure Pipelines (Tier 2) ----------------------------------------------

type azure struct{}

func (azure) Name() string                    { return "azure" }
func (azure) Tier() Tier                      { return TierLint }
func (azure) Detect(r string) bool            { return fileExists(r, "azure-pipelines.yml") }
func (azure) WorkflowFiles(r string) []string { return glob(r, "azure-pipelines.yml") }
func (azure) Lint(ctx context.Context, d Deps, r string) result.Result {
	return brokerTool(ctx, d, r, "workflow-lint", "yamllint", "azure-pipelines.yml")
}
func (azure) RunWorkflows(ctx context.Context, d Deps, r string, o RunOpts) result.Result {
	return skip("workflow-run", "azure: local run not supported; lint covered")
}

// --- Jenkins (Tier 2) ------------------------------------------------------

type jenkins struct{}

func (jenkins) Name() string                    { return "jenkins" }
func (jenkins) Tier() Tier                      { return TierLint }
func (jenkins) Detect(r string) bool            { return fileExists(r, "Jenkinsfile") }
func (jenkins) WorkflowFiles(r string) []string { return glob(r, "Jenkinsfile") }
func (jenkins) Lint(ctx context.Context, d Deps, r string) result.Result {
	return hostTool(ctx, d, r, "workflow-lint", "jenkins-cli", "declarative-linter")
}
func (jenkins) RunWorkflows(ctx context.Context, d Deps, r string, o RunOpts) result.Result {
	return skip("workflow-run", "jenkins: local run not supported; lint covered")
}

// --- Travis CI (Tier 2) ----------------------------------------------------

type travis struct{}

func (travis) Name() string                    { return "travis" }
func (travis) Tier() Tier                      { return TierLint }
func (travis) Detect(r string) bool            { return fileExists(r, ".travis.yml") }
func (travis) WorkflowFiles(r string) []string { return glob(r, ".travis.yml") }
func (travis) Lint(ctx context.Context, d Deps, r string) result.Result {
	if res := hostTool(ctx, d, r, "workflow-lint", "travis", "lint", "--no-interactive"); res.Status != result.StatusSkip {
		return res
	}
	return brokerTool(ctx, d, r, "workflow-lint", "yamllint", ".travis.yml")
}
func (travis) RunWorkflows(ctx context.Context, d Deps, r string, o RunOpts) result.Result {
	return skip("workflow-run", "travis: local run not supported; lint covered")
}

// --- Buildkite (Tier 2) ----------------------------------------------------

type buildkite struct{}

func (buildkite) Name() string { return "buildkite" }
func (buildkite) Tier() Tier   { return TierLint }
func (buildkite) Detect(r string) bool {
	return len(glob(r, ".buildkite/pipeline.yml", ".buildkite/pipeline.yaml")) > 0
}
func (buildkite) WorkflowFiles(r string) []string {
	return glob(r, ".buildkite/pipeline.yml", ".buildkite/pipeline.yaml")
}
func (buildkite) Lint(ctx context.Context, d Deps, r string) result.Result {
	return brokerTool(ctx, d, r, "workflow-lint", "yamllint", ".buildkite/")
}
func (buildkite) RunWorkflows(ctx context.Context, d Deps, r string, o RunOpts) result.Result {
	return skip("workflow-run", "buildkite: local run not supported; lint covered")
}
