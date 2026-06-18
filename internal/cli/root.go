// Package cli implements probaci's Cobra command tree. Commands are thin: they
// parse flags, build a pkg/probaci.App, and drive the engine. Output styling is
// delegated to internal/ui; machine output to internal/report.
package cli

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/simtabi/probaci/internal/ui"
	"github.com/simtabi/probaci/internal/version"
	"github.com/simtabi/probaci/pkg/probaci"
	"github.com/spf13/cobra"
)

// globals holds values bound to persistent flags.
type globals struct {
	config    string
	only      string
	skip      string
	repos     string
	chdir     string
	jobs      int
	platform  string
	dryRun    bool
	ci        bool
	jsonOut   bool
	quiet     bool
	noDocker  bool
	pull      string
	backend   string
	fullImage bool
	verbose   int
	cmdName   string
}

var g globals

// rootCmd is the base command.
var rootCmd = &cobra.Command{
	Use:           "probaci [command]",
	Short:         "Prove your CI pipeline before you push",
	Long:          "probaci (Latin probāre, \"to test/prove\") runs the same checks your CI runs, locally, with every tool brokered through a container.",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		g.cmdName = cmd.Name()
	},
	// Bare `probaci` shows the masthead, then usage.
	Run: func(cmd *cobra.Command, args []string) {
		if !g.jsonOut {
			fmt.Fprintln(cmd.OutOrStdout(), theme().Banner(version.Short()))
		}
		_ = cmd.Help()
	},
}

// Execute runs the command tree.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&g.config, "config", "", "path to a project config file (overrides ./probaci.json)")
	pf.StringVar(&g.only, "only", "", "run only these stages (comma-separated)")
	pf.StringVar(&g.skip, "skip", "", "skip these stages (comma-separated)")
	pf.StringVar(&g.repos, "repos", os.Getenv("PROBACI_REPOS"), "target repos (comma-separated; alternative to positional paths; env PROBACI_REPOS)")
	pf.StringVarP(&g.chdir, "chdir", "C", "", "run as if started in this directory")
	pf.IntVar(&g.jobs, "jobs", 1, "number of repositories to process concurrently")
	pf.StringVar(&g.platform, "platform", "", "restrict to a single CI platform by name")
	pf.BoolVar(&g.dryRun, "dry-run", false, "print what would run without executing")
	pf.BoolVar(&g.ci, "ci", false, "non-interactive, plain output (use inside CI too)")
	pf.BoolVar(&g.jsonOut, "json", false, "emit machine-readable JSON")
	pf.BoolVarP(&g.quiet, "quiet", "q", false, "suppress progress output; print only the final summary")
	pf.BoolVar(&g.noDocker, "no-docker", false, "disable the container broker (degraded mode)")
	pf.StringVar(&g.pull, "pull", "", "image pull policy: missing|always|never")
	pf.StringVar(&g.backend, "backend", "", "container backend: auto|docker|rootless|podman")
	pf.BoolVar(&g.fullImage, "full-image", false, "use the heavy runner image for workflow-run")
	pf.CountVarP(&g.verbose, "verbose", "v", "increase log verbosity (repeatable)")

	rootCmd.AddCommand(
		runCmd, doctorCmd, initCmd, tuiCmd, stageCmd, platformsCmd,
		vcsCmd, toolsCmd, docsCmd, logsCmd, cleanCmd, configCmd, versionCmd,
		installCmd, uninstallCmd,
	)
}

// theme builds a UI theme honoring --ci and TTY detection.
func theme() *ui.Theme {
	isTTY := isatty.IsTerminal(os.Stdout.Fd())
	return ui.Detect(isTTY, g.ci)
}

// newApp constructs the wired App for the given primary project directory.
func newApp(projectDir string) (*probaci.App, error) {
	// The file log defaults to Info (it's a per-run file, not console noise);
	// -v raises it to Debug. Console output is governed separately by flags.
	level := slog.LevelInfo
	if g.verbose >= 1 {
		level = slog.LevelDebug
	}
	return probaci.New(probaci.Options{
		ProjectDir: projectDir,
		ConfigPath: g.config,
		NoDocker:   g.noDocker,
		Backend:    g.backend,
		Pull:       g.pull,
		LogLevel:   level,
		Command:    g.cmdName,
	})
}

// csv splits a comma-separated flag value into a trimmed, non-empty slice.
func csv(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
