package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/simtabi/probaci/internal/config"
	"github.com/simtabi/probaci/internal/discover"
)

// resolveTargets turns positional path args, --repos, -C, and workspace
// members into a validated, absolute list of repository paths.
//
// Rule (see plan): positional args are always paths; stage selection is via
// flags. Passing both positional paths and --repos is an error.
func resolveTargets(args []string) ([]string, error) {
	reposFlag := csv(g.repos)
	if len(args) > 0 && len(reposFlag) > 0 {
		return nil, usageErr(fmt.Errorf("pass repositories either as positional paths or via --repos, not both"))
	}

	raw := args
	if len(raw) == 0 {
		raw = reposFlag
	}
	if len(raw) == 0 {
		// Default to the chdir target or the current directory; ddev-style, walk
		// up to the repo root so probaci works from any subdirectory. Expand
		// workspace members if the discovered root declares any.
		base := g.chdir
		if base == "" {
			base = "."
		}
		if root, ok := discover.Root(base); ok {
			base = root
		}
		if members, ok := workspaceMembers(base); ok {
			raw = members
		} else {
			raw = []string{base}
		}
	}

	seen := map[string]bool{}
	var out []string
	for _, p := range raw {
		if g.chdir != "" && !filepath.IsAbs(p) {
			p = filepath.Join(g.chdir, p)
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, usageErr(fmt.Errorf("resolve %q: %w", p, err))
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, usageErr(fmt.Errorf("target %q: %w", p, err))
		}
		if !info.IsDir() {
			return nil, usageErr(fmt.Errorf("target %q is not a directory", p))
		}
		if !seen[abs] {
			seen[abs] = true
			out = append(out, abs)
		}
	}
	return out, nil
}

// workspaceMembers expands the "members" globs from a root probaci.json in dir.
// Returns (paths, true) when members are declared.
func workspaceMembers(dir string) ([]string, bool) {
	cfgPath := filepath.Join(dir, config.ProjectFileName)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, false
	}
	cfg, err := config.Parse(data)
	if err != nil || len(cfg.Members) == 0 {
		return nil, false
	}
	var out []string
	for _, pattern := range cfg.Members {
		matches, _ := filepath.Glob(filepath.Join(dir, pattern))
		for _, m := range matches {
			if info, err := os.Stat(m); err == nil && info.IsDir() {
				out = append(out, m)
			}
		}
	}
	return out, len(out) > 0
}
