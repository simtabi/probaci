package cli

import (
	"context"
	"fmt"

	"github.com/simtabi/probaci/internal/platform"
	"github.com/simtabi/probaci/internal/report"
	"github.com/simtabi/probaci/internal/stage"
	"github.com/spf13/cobra"
)

var stageCmd = &cobra.Command{
	Use:   "stage <name> [PATH...]",
	Short: "Run a single stage (sugar for `run --only <name>`)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		targets, err := resolveTargets(args[1:])
		if err != nil {
			return err
		}
		app, err := newApp(targets[0])
		if err != nil {
			return usageErr(err)
		}
		defer app.Close()

		t := theme()
		opts := stage.RunOptions{
			Repos:    targets,
			Only:     []string{name},
			Jobs:     g.jobs,
			Platform: g.platform,
			RunOpts:  platform.RunOpts{FullImage: g.fullImage, AllowSocketMount: app.Cfg.Docker.SocketMountAllowed()},
		}
		agg := app.Engine.Run(context.Background(), opts, makeObserver(t))
		if g.jsonOut {
			data, _ := report.JSON(agg)
			fmt.Println(string(data))
		} else {
			fmt.Print(report.Text(agg, t))
		}
		if agg.Failed() {
			return failure(fmt.Errorf("stage %q failed", name))
		}
		return nil
	},
}
