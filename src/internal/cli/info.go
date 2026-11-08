package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/simtabi/probaci"
	"github.com/simtabi/probaci/internal/paths"
	"github.com/simtabi/probaci/internal/platform"
	"github.com/simtabi/probaci/internal/tool"
	"github.com/simtabi/probaci/internal/ui"
	"github.com/simtabi/probaci/internal/vcs"
	"github.com/simtabi/probaci/internal/version"
	"github.com/spf13/cobra"
)

var platformsCmd = &cobra.Command{
	Use:   "platforms [PATH]",
	Short: "List supported CI platforms and which are detected here",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := dirArg(args)
		detected := map[string]bool{}
		for _, p := range platform.Detect(dir) {
			detected[p.Name()] = true
		}
		t := theme()
		var rows [][2]string
		for _, p := range platform.All() {
			mark := t.Dim("available")
			if detected[p.Name()] {
				mark = t.Pass("detected")
			}
			rows = append(rows, [2]string{fmt.Sprintf("%s (%s)", p.Name(), p.Tier()), mark})
		}
		fmt.Println(t.Box("CI platforms", t.KeyValue(rows)))
		return nil
	},
}

var vcsCmd = &cobra.Command{
	Use:   "vcs [PATH]",
	Short: "Show the detected version-control provider(s)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := dirArg(args)
		t := theme()
		active := "none"
		if v := vcs.Detect(dir); v != nil {
			active = v.Name()
		}
		fmt.Println(t.Box("version control", t.KeyValue([][2]string{
			{"detected", t.Pass(active)},
			{"supported", strings.Join(vcs.Supported(), ", ")},
		})))
		return nil
	},
}

var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "List the tool registry and resolved images",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp(".")
		if err != nil {
			return usageErr(err)
		}
		defer app.Close()
		names := tool.Names()
		sort.Strings(names)
		t := theme()

		// Machine-readable form (used by scripts/pin-digests.sh and tooling).
		if g.jsonOut {
			type toolJSON struct {
				Name   string `json:"name"`
				Image  string `json:"image"`
				Tag    string `json:"tag"`
				Ref    string `json:"ref"`
				Pinned bool   `json:"pinned"`
			}
			out := make([]toolJSON, 0, len(names))
			for _, n := range names {
				r, err := app.Registry.Resolve(n)
				if err != nil {
					continue
				}
				out = append(out, toolJSON{n, r.Image, r.Tag, r.Ref(), strings.Contains(r.Ref(), "@sha256:")})
			}
			data, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return failure(err)
			}
			fmt.Println(string(data))
			return nil
		}

		var rows [][2]string
		for _, n := range names {
			resolved, err := app.Registry.Resolve(n)
			if err != nil {
				continue
			}
			ref := resolved.Ref()
			trust := t.Warn("advisory: unpinned")
			if strings.Contains(ref, "@sha256:") {
				trust = t.Pass("pinned")
			}
			rows = append(rows, [2]string{n, t.Dim(ref) + "  " + trust})
		}
		fmt.Println(t.Box("tool registry", t.KeyValue(rows)))
		return nil
	},
}

var docsCmd = &cobra.Command{
	Use:   "docs [topic]",
	Short: "Render probaci documentation in the terminal",
	Long: "Render an embedded documentation topic in the terminal. With no topic, " +
		"shows the available topics. Topics: " + strings.Join(probaci.DocTopics(), ", ") + ".",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		t := theme()
		if len(args) == 0 {
			var rows [][2]string
			for _, topic := range probaci.DocTopics() {
				rows = append(rows, [2]string{topic, t.Dim("probaci docs " + topic)})
			}
			rows = append(rows, [2]string{"online", "https://opensource.simtabi.com/documentation/probaci"})
			fmt.Println(t.Box("probaci docs — topics", t.KeyValue(rows)))
			return nil
		}
		topic := args[0]
		md, ok := probaci.Doc(topic)
		if !ok {
			return usageErr(fmt.Errorf("unknown docs topic %q (try: %s)", topic, strings.Join(probaci.DocTopics(), ", ")))
		}
		out, err := renderMarkdown(md, t)
		if err != nil {
			fmt.Print(md) // fall back to raw markdown
			return nil
		}
		fmt.Print(out)
		return nil
	},
}

// renderMarkdown renders markdown for the terminal with glamour, choosing a
// no-color style when color is disabled (NO_COLOR/--ci/non-TTY).
func renderMarkdown(md string, t *ui.Theme) (string, error) {
	style := "notty"
	if t.Color {
		style = "auto"
	}
	r, err := glamour.NewTermRenderer(glamour.WithStandardStyle(style), glamour.WithWordWrap(90))
	if err != nil {
		return "", err
	}
	return r.Render(md)
}

var logsSelf bool

var logsCmd = &cobra.Command{
	Use:   "logs [PATH]",
	Short: "Show failing steps from the latest remote CI run (--self for probaci's own logs)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		t := theme()
		if logsSelf {
			l := paths.Resolve()
			fmt.Println(t.Box("probaci logs", t.KeyValue([][2]string{
				{"dir", l.Logs},
				{"tail", "tail -f " + filepath.Join(l.Logs, "*", "*.log")},
			})))
			return nil
		}

		dir := dirArg(args)
		plats := platform.Detect(dir)
		if len(plats) == 0 {
			fmt.Println(t.Dim("no CI platform detected here; use --self for probaci's own logs"))
			return nil
		}
		ran := false
		for _, p := range plats {
			switch p.Name() {
			case "github":
				if hostAvailable("gh") {
					ran = true
					fmt.Println(t.Heading("github — latest failed steps"))
					streamHost(dir, "gh", "run", "view", "--log-failed")
				}
			case "gitlab":
				if hostAvailable("glab") {
					ran = true
					fmt.Println(t.Heading("gitlab — latest pipeline"))
					streamHost(dir, "glab", "ci", "status")
				}
			}
		}
		if !ran {
			fmt.Println(t.Dim("install and authenticate the GitHub CLI (gh) or GitLab CLI (glab) to pull remote-CI failures; or use --self"))
		}
		return nil
	},
}

func hostAvailable(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// streamHost runs a host CLI in dir, streaming its output to the terminal.
func streamHost(dir, bin string, args ...string) {
	c := exec.Command(bin, args...) // #nosec G204 -- fixed CLI name; args are constant literals
	c.Dir = dir
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	_ = c.Run()
}

var cleanAll bool

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Prune probaci-labeled containers and stray temp dirs",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp(".")
		if err != nil {
			return usageErr(err)
		}
		defer app.Close()
		t := theme()
		if app.Broker == nil {
			return dockerErr(fmt.Errorf("no container runtime available to clean"))
		}
		ctx := context.Background()
		if err := app.Broker.Available(ctx); err != nil {
			return dockerErr(err)
		}
		n, err := app.Broker.Prune(ctx, cleanAll)
		if err != nil {
			return failure(err)
		}
		removed := pruneTempDirs()
		scope := "your"
		if cleanAll {
			scope = "all users'"
		}
		fmt.Printf("%s pruned %d %s leaked container(s), %d stray temp dir(s)\n",
			t.Pass("ok"), n, scope, removed)
		return nil
	},
}

// pruneTempDirs removes stray probaci-clean-* directories left by crashed runs.
func pruneTempDirs() int {
	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "probaci-clean-*"))
	n := 0
	for _, m := range matches {
		if err := os.RemoveAll(m); err == nil {
			n++
		}
	}
	return n
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		t := theme()
		if t.Color && !g.jsonOut {
			fmt.Println(t.Banner(version.Short()))
		}
		fmt.Println(version.String())
		return nil
	},
}

func dirArg(args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	if g.chdir != "" {
		return g.chdir
	}
	return "."
}

func init() {
	logsCmd.Flags().BoolVar(&logsSelf, "self", false, "show probaci's own log location")
	cleanCmd.Flags().BoolVar(&cleanAll, "all", false, "prune all users' probaci containers (admin)")
}
