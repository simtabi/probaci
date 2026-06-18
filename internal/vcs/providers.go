package vcs

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// run executes a VCS binary in dir with no shell and returns trimmed stdout.
func run(ctx context.Context, dir, bin string, args ...string) (string, error) {
	if _, err := exec.LookPath(bin); err != nil {
		return "", fmt.Errorf("%s not installed", bin)
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", bin, strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// --- git -------------------------------------------------------------------

type git struct{}

func (git) Name() string         { return "git" }
func (git) Detect(d string) bool { return hasDir(d, ".git") || hasEntry(d, ".git") }

func (git) IsDirty(ctx context.Context, d string) (bool, error) {
	out, err := run(ctx, d, "git", "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

func (git) CurrentRev(ctx context.Context, d string) (string, error) {
	return run(ctx, d, "git", "rev-parse", "HEAD")
}

func (g git) ExportCommitted(ctx context.Context, d, dest string) error {
	rev, err := g.CurrentRev(ctx, d)
	if err != nil {
		return err
	}
	if _, err := run(ctx, "", "git", "clone", "--quiet", "--no-hardlinks", d, dest); err != nil {
		return err
	}
	_, err = run(ctx, dest, "git", "checkout", "--quiet", rev)
	return err
}

// --- mercurial -------------------------------------------------------------

type mercurial struct{}

func (mercurial) Name() string         { return "hg" }
func (mercurial) Detect(d string) bool { return hasDir(d, ".hg") }

func (mercurial) IsDirty(ctx context.Context, d string) (bool, error) {
	out, err := run(ctx, d, "hg", "status")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

func (mercurial) CurrentRev(ctx context.Context, d string) (string, error) {
	return run(ctx, d, "hg", "id", "-i")
}

func (mercurial) ExportCommitted(ctx context.Context, d, dest string) error {
	_, err := run(ctx, d, "hg", "archive", dest)
	return err
}

// --- subversion ------------------------------------------------------------

type subversion struct{}

func (subversion) Name() string         { return "svn" }
func (subversion) Detect(d string) bool { return hasDir(d, ".svn") }

func (subversion) IsDirty(ctx context.Context, d string) (bool, error) {
	out, err := run(ctx, d, "svn", "status")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (subversion) CurrentRev(ctx context.Context, d string) (string, error) {
	return run(ctx, d, "svnversion", ".")
}

func (subversion) ExportCommitted(ctx context.Context, d, dest string) error {
	_, err := run(ctx, "", "svn", "export", "--force", d, dest)
	return err
}

// --- perforce (Helix) ------------------------------------------------------

type perforce struct{}

func (perforce) Name() string         { return "p4" }
func (perforce) Detect(d string) bool { return hasEntry(d, ".p4config") || hasEntry(d, "P4CONFIG") }

func (perforce) IsDirty(ctx context.Context, d string) (bool, error) {
	out, err := run(ctx, d, "p4", "opened")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (perforce) CurrentRev(ctx context.Context, d string) (string, error) {
	return run(ctx, d, "p4", "changes", "-m1", "#have")
}

func (perforce) ExportCommitted(ctx context.Context, d, dest string) error {
	return fmt.Errorf("p4: clean export must target a configured client; run probaci inside a synced workspace")
}

// --- fossil ----------------------------------------------------------------

type fossil struct{}

func (fossil) Name() string         { return "fossil" }
func (fossil) Detect(d string) bool { return hasEntry(d, ".fslckout") || hasEntry(d, "_FOSSIL_") }

func (fossil) IsDirty(ctx context.Context, d string) (bool, error) {
	out, err := run(ctx, d, "fossil", "changes")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (fossil) CurrentRev(ctx context.Context, d string) (string, error) {
	return run(ctx, d, "fossil", "status")
}

func (fossil) ExportCommitted(ctx context.Context, d, dest string) error {
	_, err := run(ctx, d, "fossil", "tarball", "--name", "export", "current", dest)
	return err
}

// --- bazaar / breezy -------------------------------------------------------

type bazaar struct{}

func (bazaar) Name() string         { return "bzr" }
func (bazaar) Detect(d string) bool { return hasDir(d, ".bzr") }

func (bazaar) IsDirty(ctx context.Context, d string) (bool, error) {
	out, err := run(ctx, d, "bzr", "status")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (bazaar) CurrentRev(ctx context.Context, d string) (string, error) {
	return run(ctx, d, "bzr", "revno")
}

func (bazaar) ExportCommitted(ctx context.Context, d, dest string) error {
	_, err := run(ctx, d, "bzr", "export", dest)
	return err
}

// --- sapling ---------------------------------------------------------------

type sapling struct{}

func (sapling) Name() string         { return "sl" }
func (sapling) Detect(d string) bool { return hasDir(d, ".sl") }

func (sapling) IsDirty(ctx context.Context, d string) (bool, error) {
	out, err := run(ctx, d, "sl", "status")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (sapling) CurrentRev(ctx context.Context, d string) (string, error) {
	return run(ctx, d, "sl", "id", "-i")
}

func (sapling) ExportCommitted(ctx context.Context, d, dest string) error {
	_, err := run(ctx, d, "sl", "archive", dest)
	return err
}

// --- cvs (detect-only) -----------------------------------------------------

type cvs struct{}

func (cvs) Name() string         { return "cvs" }
func (cvs) Detect(d string) bool { return hasDir(d, "CVS") }

func (cvs) IsDirty(context.Context, string) (bool, error) {
	return false, fmt.Errorf("cvs: dirty detection not supported")
}

func (cvs) CurrentRev(context.Context, string) (string, error) {
	return "", fmt.Errorf("cvs: revision lookup not supported")
}

func (cvs) ExportCommitted(context.Context, string, string) error {
	return fmt.Errorf("cvs: clean export not supported (detect-only)")
}
