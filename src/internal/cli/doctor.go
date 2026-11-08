package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/simtabi/probaci/internal/detect"
	"github.com/simtabi/probaci/internal/platform"
	"github.com/simtabi/probaci/internal/vcs"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor [PATH]",
	Short: "Report runtime, detected platforms/languages, and tool readiness",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) == 1 {
			dir = args[0]
		}
		app, err := newApp(dir)
		if err != nil {
			return usageErr(err)
		}
		defer app.Close()
		t := theme()

		var pairs [][2]string
		// Container runtime.
		if app.Broker == nil {
			pairs = append(pairs, [2]string{"runtime", t.Warn("disabled (--no-docker or unavailable)")})
		} else if err := app.CheckBroker(context.Background()); err != nil {
			pairs = append(pairs, [2]string{"runtime", t.Fail(string(app.Broker.Backend()) + ": not reachable")})
		} else {
			pairs = append(pairs, [2]string{"runtime", t.Pass(string(app.Broker.Backend()) + " (" + app.Broker.Bin() + ")")})
		}

		// Detected project.
		proj := detect.DetectProject(dir)
		var langs []string
		for _, l := range proj.Languages {
			langs = append(langs, l.Name)
		}
		if len(langs) == 0 {
			langs = []string{"none"}
		}
		pairs = append(pairs, [2]string{"languages", strings.Join(langs, ", ")})

		// VCS.
		v := vcs.Detect(dir)
		vName := "none"
		if v != nil {
			vName = v.Name()
		}
		pairs = append(pairs, [2]string{"vcs", vName})

		// Platforms.
		var plats []string
		for _, p := range platform.Detect(dir) {
			plats = append(plats, fmt.Sprintf("%s(%s)", p.Name(), p.Tier()))
		}
		if len(plats) == 0 {
			plats = []string{"none"}
		}
		pairs = append(pairs, [2]string{"platforms", strings.Join(plats, ", ")})

		// Config provenance.
		var src []string
		for _, s := range app.Sources {
			src = append(src, string(s))
		}
		pairs = append(pairs, [2]string{"config", strings.Join(src, " -> ")})

		fmt.Println(t.Box("probaci doctor — "+dir, t.KeyValue(pairs)))
		return nil
	},
}
