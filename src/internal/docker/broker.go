// Package docker is the container broker: probaci runs every CI tool and
// language toolchain inside a pinned container so the only hard local
// dependency is a container runtime. The broker supports Docker, rootless
// Docker, and Podman, and runs containers with least privilege by default.
package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/simtabi/probaci/internal/config"
)

// Broker executes tools in containers via a chosen backend binary.
type Broker struct {
	bin     string // "docker" or "podman"
	backend config.Backend
	cfg     config.Docker
	sec     config.Security
	labels  map[string]string // ownership labels applied to every container
}

// Label keys applied to every brokered container so `clean` can reclaim
// crash-leaked containers scoped to the current user/run.
const (
	LabelMarker = "probaci"
	LabelUser   = "probaci.user"
	LabelRun    = "probaci.run"
)

// RunSpec describes a single container invocation. Commands are passed as arg
// slices only — the broker never builds a shell string, eliminating the
// injection class that affected the original bash tool.
type RunSpec struct {
	// Image is the fully-resolved reference, ideally pinned by digest.
	Image string
	// Entrypoint overrides the image entrypoint when set.
	Entrypoint string
	// Args are the command arguments (never shell-interpreted).
	Args []string
	// RepoMount is the host path bind-mounted at /workspace.
	RepoMount string
	// Writable mounts the repo read-write; default is read-only.
	Writable bool
	// Network grants network access ("" means none — fully offline).
	Network string
	// Env passes individual environment variables.
	Env map[string]string
	// EnvFile passes a KEY=value file (never via argv).
	EnvFile string
	// Stdout/Stderr/Stdin wire the container's streams.
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}

const workspace = "/workspace"

// New constructs a Broker, resolving the backend binary. With BackendAuto it
// prefers an available docker, then podman. labels are applied to every
// container (ownership/run attribution); may be nil.
func New(cfg config.Docker, sec config.Security, labels map[string]string) (*Broker, error) {
	bin, backend, err := resolveBackend(cfg.Backend)
	if err != nil {
		return nil, err
	}
	return &Broker{bin: bin, backend: backend, cfg: cfg, sec: sec, labels: labels}, nil
}

func resolveBackend(b config.Backend) (string, config.Backend, error) {
	switch b {
	case config.BackendDocker, config.BackendRootless:
		if _, err := exec.LookPath("docker"); err != nil {
			return "", "", fmt.Errorf("backend %q requires docker on PATH: %w", b, err)
		}
		return "docker", b, nil
	case config.BackendPodman:
		if _, err := exec.LookPath("podman"); err != nil {
			return "", "", fmt.Errorf("backend podman requires podman on PATH: %w", err)
		}
		return "podman", b, nil
	case config.BackendAuto, "":
		if _, err := exec.LookPath("docker"); err == nil {
			return "docker", config.BackendDocker, nil
		}
		if _, err := exec.LookPath("podman"); err == nil {
			return "podman", config.BackendPodman, nil
		}
		return "", "", errors.New("no container runtime found (install Docker or Podman)")
	default:
		return "", "", fmt.Errorf("unknown backend %q", b)
	}
}

// Backend reports the resolved backend.
func (b *Broker) Backend() config.Backend { return b.backend }

// Bin reports the resolved runtime binary.
func (b *Broker) Bin() string { return b.bin }

// Available verifies the runtime daemon is reachable, returning an actionable
// error when it is not.
func (b *Broker) Available(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, b.bin, "info", "--format", "{{.ServerVersion}}")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s daemon not reachable — start it and re-run: %s", b.bin, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// Run executes spec and returns the command error (non-nil on non-zero exit).
// The image-trust policy is enforced first: in strict mode an unpinned or
// unverified image blocks the run.
func (b *Broker) Run(ctx context.Context, spec RunSpec) error {
	if _, err := b.CheckImage(ctx, b.image(spec.Image)); err != nil {
		return err
	}
	args := b.buildArgs(spec)
	cmd := exec.CommandContext(ctx, b.bin, args...)
	cmd.Stdout = spec.Stdout
	cmd.Stderr = spec.Stderr
	cmd.Stdin = spec.Stdin
	return cmd.Run()
}

// Capture runs spec and returns combined stdout+stderr.
func (b *Broker) Capture(ctx context.Context, spec RunSpec) (string, error) {
	var buf bytes.Buffer
	spec.Stdout = &buf
	spec.Stderr = &buf
	err := b.Run(ctx, spec)
	return buf.String(), err
}

// Prune removes leftover probaci-labeled containers. When allUsers is false it
// scopes removal to the current user's label. Returns the number removed.
func (b *Broker) Prune(ctx context.Context, allUsers bool) (int, error) {
	filterArgs := []string{"ps", "-aq", "--filter", "label=" + LabelMarker + "=true"}
	if !allUsers {
		if u := b.labels[LabelUser]; u != "" {
			filterArgs = append(filterArgs, "--filter", "label="+LabelUser+"="+u)
		}
	}
	out, err := exec.CommandContext(ctx, b.bin, filterArgs...).Output()
	if err != nil {
		return 0, err
	}
	ids := strings.Fields(string(out))
	if len(ids) == 0 {
		return 0, nil
	}
	rmArgs := append([]string{"rm", "-f"}, ids...)
	if err := exec.CommandContext(ctx, b.bin, rmArgs...).Run(); err != nil {
		return 0, err
	}
	return len(ids), nil
}

// buildArgs assembles the hardened `run` argument vector.
func (b *Broker) buildArgs(spec RunSpec) []string {
	args := []string{
		"run", "--rm",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
	}
	for k, v := range b.labels {
		args = append(args, "--label", k+"="+v)
	}

	// Read-only root filesystem with a writable tmpfs scratch, unless the spec
	// needs a writable workspace (e.g. clean-clone export).
	if !spec.Writable {
		args = append(args, "--read-only", "--tmpfs", "/tmp:rw,exec,nosuid,size=512m")
	}

	// Non-root execution on Linux, where host uid/gid map directly onto the
	// bind mount. Docker Desktop / Podman handle mapping on macOS and Windows.
	if runtime.GOOS == "linux" {
		args = append(args, "--user", currentUserSpec())
	}

	if b.cfg.Resources.PidsLimit > 0 {
		args = append(args, "--pids-limit", strconv.Itoa(b.cfg.Resources.PidsLimit))
	}
	if b.cfg.Resources.Memory != "" {
		args = append(args, "--memory", b.cfg.Resources.Memory)
	}
	if b.cfg.Resources.CPUs != "" {
		args = append(args, "--cpus", b.cfg.Resources.CPUs)
	}

	// Network: default to fully offline; only the caller's explicit request or
	// the configured network opens it.
	network := spec.Network
	if network == "" {
		network = "none"
		if b.cfg.Network != "" {
			network = b.cfg.Network
		}
	}
	args = append(args, "--network", network)

	if spec.RepoMount != "" {
		mount := spec.RepoMount + ":" + workspace
		if !spec.Writable {
			mount += ":ro"
		}
		args = append(args, "-v", mount, "-w", workspace)
	}

	if spec.EnvFile != "" {
		args = append(args, "--env-file", spec.EnvFile)
	}
	for k, v := range spec.Env {
		args = append(args, "-e", k+"="+v)
	}
	if spec.Entrypoint != "" {
		args = append(args, "--entrypoint", spec.Entrypoint)
	}

	args = append(args, b.image(spec.Image))
	args = append(args, spec.Args...)
	return args
}

// image applies the optional private-registry mirror rewrite. It prefixes the
// mirror for Docker Hub images — both bare official images ("golang") and
// org/name images ("aquasec/trivy") — while leaving images that already name a
// registry host (a first path segment containing "." or ":", e.g.
// "ghcr.io/...") untouched.
func (b *Broker) image(ref string) string {
	mirror := b.sec.RegistryMirror
	if mirror == "" {
		return ref
	}
	first := ref
	if i := strings.IndexByte(ref, '/'); i >= 0 {
		first = ref[:i]
	}
	hasRegistryHost := strings.ContainsAny(first, ".:")
	if hasRegistryHost || strings.HasPrefix(ref, mirror+"/") {
		return ref // already a fully-qualified or already-mirrored reference
	}
	return mirror + "/" + ref
}
