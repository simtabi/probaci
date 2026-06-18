package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/mattn/go-isatty"
	"github.com/simtabi/probaci/internal/config"
	"github.com/simtabi/probaci/internal/detect"
	"github.com/spf13/cobra"
)

var (
	initForce bool
	initYes   bool
)

var initCmd = &cobra.Command{
	Use:   "init [PATH]",
	Short: "Write a probaci.json into the repository, detected from its contents",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) == 1 {
			dir = args[0]
		}
		path := filepath.Join(dir, config.ProjectFileName)

		if _, err := os.Stat(path); err == nil && !initForce {
			return usageErr(fmt.Errorf("%s already exists (use --force to overwrite)", path))
		}

		// Interactive form when attached to a terminal; otherwise fall back to
		// the detected defaults (CI/scripts).
		if interactive() {
			cfg, err := initForm(dir)
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					fmt.Println(theme().Dim("aborted"))
					return nil
				}
				return failure(err)
			}
			if err := config.Write(path, cfg); err != nil {
				return failure(err)
			}
		} else if err := config.Init(path, initForce); err != nil {
			return usageErr(err)
		}

		fmt.Printf("%s wrote %s\n", theme().Pass("ok"), path)
		fmt.Println(theme().Dim("review it, then run: probaci run"))
		return nil
	},
}

// interactive reports whether to present a form (TTY on both ends, not --ci/--yes).
func interactive() bool {
	if g.ci || initYes {
		return false
	}
	return isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd())
}

// initForm presents the configuration form seeded from detection, returning the
// chosen config.
func initForm(dir string) (config.Config, error) {
	cfg := config.Default()

	// Seed stage selection from the enabled defaults.
	stages := cfg.EnabledStages()
	stageOpts := huh.NewOptions(config.DefaultStageOrder...)

	// Seed language override from detection.
	detected := detect.DetectProject(dir)
	var langs []string
	for _, l := range detected.Languages {
		langs = append(langs, l.Name)
	}
	langOpts := huh.NewOptions("go", "python", "node", "rust", "ruby", "java")

	backend := string(config.BackendAuto)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Stages to enable").
				Options(stageOpts...).
				Value(&stages),
			huh.NewMultiSelect[string]().
				Title("Languages (override auto-detection; leave as detected if unsure)").
				Options(langOpts...).
				Value(&langs),
			huh.NewSelect[string]().
				Title("Container backend").
				Options(
					huh.NewOption("auto (docker, else podman)", string(config.BackendAuto)),
					huh.NewOption("docker", string(config.BackendDocker)),
					huh.NewOption("rootless docker", string(config.BackendRootless)),
					huh.NewOption("podman", string(config.BackendPodman)),
				).
				Value(&backend),
		),
	)
	if err := form.Run(); err != nil {
		return config.Config{}, err
	}

	enabled := map[string]bool{}
	for _, s := range stages {
		enabled[s] = true
	}
	for i := range cfg.Stages {
		cfg.Stages[i].Enabled = enabled[cfg.Stages[i].Name]
	}
	cfg.Project.Languages = langs
	cfg.Docker.Backend = config.Backend(backend)
	return cfg, nil
}

func init() {
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite an existing config (backs up first)")
	initCmd.Flags().BoolVarP(&initYes, "yes", "y", false, "skip the interactive form; write detected defaults")
}
