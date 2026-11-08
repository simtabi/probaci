package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/simtabi/probaci/internal/config"
	"github.com/simtabi/probaci/internal/paths"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage probaci configuration (path/show/init/edit/validate/reset/restore/migrate)",
}

var configForce bool
var restoreFrom string

func init() {
	configCmd.PersistentFlags().BoolVar(&configForce, "force", false, "skip confirmation backups guard")

	configPathCmd := &cobra.Command{
		Use:   "path",
		Short: "Print the resolved config home and active files",
		RunE: func(cmd *cobra.Command, args []string) error {
			l := paths.Resolve()
			t := theme()
			fmt.Println(t.Box("config home", t.KeyValue([][2]string{
				{"home", l.Home},
				{"config", existsMark(t, l.Config)},
				{"tools", existsMark(t, l.Tools)},
				{"logs", l.Logs},
				{"cache", l.Cache},
				{"secrets", l.Secrets},
			})))
			return nil
		},
	}

	configShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Print the effective merged configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(".")
			if err != nil {
				return usageErr(err)
			}
			defer app.Close()
			data, err := config.Marshal(app.Cfg)
			if err != nil {
				return failure(err)
			}
			fmt.Println(string(data))
			if !g.jsonOut {
				fmt.Fprintln(os.Stderr, theme().Dim("sources: "+joinSources(app.Sources)))
			}
			return nil
		},
	}

	configInitCmd := &cobra.Command{
		Use:   "init",
		Short: "Write the user-global config from defaults",
		RunE: func(cmd *cobra.Command, args []string) error {
			l := paths.Resolve()
			if err := l.EnsureDirs(); err != nil {
				return failure(err)
			}
			if err := config.Locked(l.Lock, func() error {
				if e := config.Init(l.Config, configForce); e != nil {
					return e
				}
				return config.WriteSchema(l.Schema)
			}); err != nil {
				return usageErr(err)
			}
			fmt.Printf("%s wrote %s (+ schema)\n", theme().Pass("ok"), l.Config)
			return nil
		},
	}

	configSchemaCmd := &cobra.Command{
		Use:   "schema",
		Short: "Print the probaci.json JSON Schema (draft 2020-12)",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Print(string(config.Schema()))
			return nil
		},
	}

	configEditCmd := &cobra.Command{
		Use:   "edit",
		Short: "Open the user-global config in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			l := paths.Resolve()
			if _, err := os.Stat(l.Config); os.IsNotExist(err) {
				if err := config.Init(l.Config, false); err != nil {
					return failure(err)
				}
			}
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}
			// #nosec G702 -- opens the user's own $EDITOR on their own config file
			c := exec.Command(editor, l.Config)
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			return c.Run()
		},
	}

	configValidateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate the effective config against the schema",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(".")
			if err != nil {
				return usageErr(err)
			}
			defer app.Close()
			fmt.Println(theme().Pass("config is valid"))
			return nil
		},
	}

	configResetCmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset the user-global config to factory defaults (backs up first)",
		RunE: func(cmd *cobra.Command, args []string) error {
			l := paths.Resolve()
			var backup string
			err := config.Locked(l.Lock, func() error {
				var e error
				backup, e = config.Reset(l.Config)
				return e
			})
			if err != nil {
				return failure(err)
			}
			t := theme()
			if backup != "" {
				fmt.Printf("%s backed up to %s\n", t.Dim("note"), backup)
			}
			fmt.Printf("%s reset %s to defaults\n", t.Pass("ok"), l.Config)
			return nil
		},
	}

	configRestoreCmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore the user-global config from its most recent backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			l := paths.Resolve()
			var used string
			err := config.Locked(l.Lock, func() error {
				var e error
				used, e = config.Restore(l.Config, restoreFrom)
				return e
			})
			if err != nil {
				return failure(err)
			}
			fmt.Printf("%s restored %s from %s\n", theme().Pass("ok"), l.Config, used)
			return nil
		},
	}
	configRestoreCmd.Flags().StringVar(&restoreFrom, "from", "", "restore from a specific backup file")

	configMigrateCmd := &cobra.Command{
		Use:   "migrate [ci-local.config.json]",
		Short: "Import a legacy ci-local.config.json into a probaci.json",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			legacy := "ci-local.config.json"
			if len(args) == 1 {
				legacy = args[0]
			}
			cfg, err := config.Migrate(legacy)
			if err != nil {
				return usageErr(err)
			}
			out := config.ProjectFileName
			if err := config.Write(out, cfg); err != nil {
				return failure(err)
			}
			fmt.Printf("%s migrated %s -> %s\n", theme().Pass("ok"), legacy, out)
			return nil
		},
	}

	configCmd.AddCommand(configPathCmd, configShowCmd, configInitCmd, configEditCmd,
		configValidateCmd, configResetCmd, configRestoreCmd, configMigrateCmd, configSchemaCmd)
}

func existsMark(t themeMark, path string) string {
	if _, err := os.Stat(path); err == nil {
		return path + " " + t.Pass("(present)")
	}
	return path + " " + t.Dim("(default)")
}

// themeMark is the minimal styling interface used by existsMark.
type themeMark interface {
	Pass(string) string
	Dim(string) string
}

func joinSources(srcs []config.Source) string {
	out := ""
	for i, s := range srcs {
		if i > 0 {
			out += " -> "
		}
		out += string(s)
	}
	return out
}
