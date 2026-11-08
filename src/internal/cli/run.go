package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/simtabi/probaci/internal/platform"
	"github.com/simtabi/probaci/internal/report"
	"github.com/simtabi/probaci/internal/result"
	"github.com/simtabi/probaci/internal/stage"
	"github.com/simtabi/probaci/internal/ui"
	"github.com/simtabi/probaci/pkg/probaci"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run [PATH...]",
	Short: "Run the pipeline on one or more repositories (default: current dir)",
	Long: "Run the configured pipeline. Positional arguments are repository paths " +
		"(space-separated, glob-friendly); stage selection is via --only/--skip.",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets, err := resolveTargets(args)
		if err != nil {
			return err
		}
		base := targets[0]

		app, err := newApp(base)
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

		if g.dryRun {
			return printPlan(app, targets, opts)
		}

		ctx := context.Background()
		if err := app.CheckBroker(ctx); err != nil && app.Broker != nil {
			fmt.Fprintln(os.Stderr, theme().Warn("warning: "+err.Error()))
		}

		app.Logger.Info("run started",
			"run", app.RunID.Short, "repos", len(targets), "jobs", g.jobs,
			"stages", strings.Join(app.Engine.StageList(opts.Only, opts.Skip), ","))

		t := theme()
		obs := makeObserver(t)
		agg := app.Engine.Run(ctx, opts, obs)

		for _, repo := range agg.Repos {
			for _, r := range repo.Results {
				app.Logger.Info("stage finished",
					"repo", repo.Path, "stage", r.Stage, "status", string(r.Status),
					"summary", r.Summary)
			}
		}
		app.Logger.Info("run finished", "run", app.RunID.Short, "failed", agg.Failed())

		if g.jsonOut {
			data, err := report.JSON(agg)
			if err != nil {
				return failure(err)
			}
			fmt.Println(string(data))
		} else {
			fmt.Print(report.Text(agg, t))
		}
		if agg.Failed() {
			return failure(fmt.Errorf("one or more stages failed"))
		}
		return nil
	},
}

// makeObserver streams stage start/finish lines unless --json or --quiet.
func makeObserver(t *ui.Theme) stage.Observer {
	if g.jsonOut || g.quiet {
		return func(stage.Event) {}
	}
	return func(e stage.Event) {
		switch {
		case e.Result != nil:
			fmt.Printf("  %s %s\n", t.Glyph(e.Result.Status), e.Stage)
		case e.Status == result.StatusRunning && e.Line == "":
			fmt.Printf("  %s %s\n", t.Glyph(result.StatusRunning), t.Dim(e.Stage))
		}
	}
}

func printPlan(app *probaci.App, targets []string, opts stage.RunOptions) error {
	t := theme()
	fmt.Println(t.Heading("plan (dry-run)"))
	for _, repo := range targets {
		fmt.Printf("  %s %s\n", t.Bold("repo"), repo)
	}
	stages := app.Engine.StageList(opts.Only, opts.Skip)
	for _, s := range stages {
		fmt.Printf("    %s %s\n", t.Dim("stage"), s)
	}
	return nil
}
