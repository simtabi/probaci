package cli

import (
	"github.com/simtabi/probaci/internal/platform"
	"github.com/simtabi/probaci/internal/stage"
	"github.com/simtabi/probaci/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui [PATH...]",
	Short: "Launch the interactive dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets, err := resolveTargets(args)
		if err != nil {
			return err
		}
		app, err := newApp(targets[0])
		if err != nil {
			return usageErr(err)
		}
		defer app.Close()

		opts := stage.RunOptions{
			Repos:    targets,
			Only:     csv(g.only),
			Skip:     csv(g.skip),
			Jobs:     g.jobs,
			Platform: g.platform,
			RunOpts:  platform.RunOpts{FullImage: g.fullImage, AllowSocketMount: app.Cfg.Docker.SocketMountAllowed()},
		}
		return tui.Run(app.Engine, opts)
	},
}
