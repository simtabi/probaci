package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/simtabi/probaci/src/internal/config"
	"github.com/simtabi/probaci/src/internal/detect"
	"github.com/simtabi/probaci/src/internal/docker"
	"github.com/simtabi/probaci/src/internal/paths"
	"github.com/simtabi/probaci/src/internal/platform"
	"github.com/simtabi/probaci/src/internal/result"
	"github.com/simtabi/probaci/src/internal/stage"
	"github.com/simtabi/probaci/src/internal/version"
	"github.com/simtabi/probaci/src/internal/web"
)

// Seams for tests: a test must not bind a real port it cannot release, and
// must never open a browser.
var (
	serveListen  web.ListenFunc = net.Listen
	serveBrowser                = openBrowser
)

type serveOptions struct {
	addr        string
	port        int
	allowRemote bool
	allowHosts  []string
	readOnly    bool
	noOpen      bool
}

var serveOpts = serveOptions{addr: "127.0.0.1", port: web.DefaultPort}

var serveCmd = &cobra.Command{
	Use:     "serve [PATH...]",
	Aliases: []string{"ui"},
	Short:   "Open the web UI on this machine",
	Long: `Open the web UI on this machine: the repositories a run covers, a form to
start a run with the stages you pick, its output as it happens, and every
earlier run.

Listens on 127.0.0.1, port 7844 by default. If that port is taken it takes the
next free one and prints the address it actually bound; --port pins a port and
fails if it is taken. The printed address carries a one-time token that signs
the browser in; without it every page answers 401.

Positional arguments and --repos name the repositories, as for run. --config,
--no-docker, --backend and --pull apply to every run started from the UI.

Opens the default browser when run by a person at a terminal: never when CI is
set, when PROBACI_NO_BROWSER=1, when stdout is not a terminal, or on Linux with
neither DISPLAY nor WAYLAND_DISPLAY. --no-open turns it off.

A non-loopback --addr is refused unless --allow-remote is passed. Prefer an
SSH tunnel: ssh -L 7844:127.0.0.1:7844 <host>. --read-only serves no route
that starts or cancels a run.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		targets, err := resolveTargets(args)
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return serveUI(ctx, cmd.OutOrStdout(), cmd.ErrOrStderr(), serveOpts, cmd.Flags().Changed("port"), targets)
	},
}

func init() {
	f := serveCmd.Flags()
	f.StringVar(&serveOpts.addr, "addr", serveOpts.addr, "address to listen on, host or host:port (a port here pins it, like --port)")
	f.IntVar(&serveOpts.port, "port", serveOpts.port, "port to listen on; when not given and the default is taken, the next free port is used")
	f.BoolVar(&serveOpts.allowRemote, "allow-remote", false, "allow a non-loopback --addr (plain HTTP; prefer an SSH tunnel)")
	f.StringArrayVar(&serveOpts.allowHosts, "allow-host", nil, "extra Host header to accept with --allow-remote, e.g. a proxy's name (repeatable)")
	f.BoolVar(&serveOpts.readOnly, "read-only", false, "serve no route that starts or cancels a run")
	f.BoolVar(&serveOpts.noOpen, "no-open", false, "do not open a browser (also PROBACI_NO_BROWSER=1)")
	rootCmd.AddCommand(serveCmd)
}

// serveUI runs the web UI until ctx is canceled or the UI's Stop button is
// pressed. portChanged says --port was given, which pins the port.
func serveUI(ctx context.Context, out, errOut io.Writer, o serveOptions, portChanged bool, targets []string) error {
	host, port, hasPort, err := splitAddr(o.addr)
	if err != nil {
		return usageErr(fmt.Errorf("--addr %q: %w", o.addr, err))
	}
	pinned := hasPort || portChanged
	if !hasPort {
		port = o.port
	}
	if port < 0 || port > 65535 {
		return usageErr(fmt.Errorf("--port %d: want 0-65535", port))
	}
	if !web.IsLoopback(host) && !o.allowRemote {
		return usageErr(fmt.Errorf("--addr %s is reachable from other machines; pass --allow-remote to mean it, "+
			"or tunnel instead: ssh -L %d:127.0.0.1:%d <host>", host, web.DefaultPort, web.DefaultPort))
	}

	ln, err := web.Listen(serveListen, host, port, pinned, web.DefaultPortAttempts)
	if err != nil {
		return failure(err)
	}
	bound := ln.Addr().(*net.TCPAddr).Port

	token := web.NewToken()
	srv, err := web.New(web.Options{
		Runner:       &appRunner{targets: targets},
		Token:        token,
		AllowedHosts: web.AllowedHosts(host, bound, o.allowHosts),
		AllowRemote:  o.allowRemote,
		ReadOnly:     o.readOnly,
		Version:      version.Short(),
		BindAddr:     ln.Addr().String(),
		HistoryDir:   web.DefaultHistoryDir(os.LookupEnv),
	})
	if err != nil {
		_ = ln.Close()
		return failure(err)
	}

	url := fmt.Sprintf("http://%s/?token=%s", net.JoinHostPort(displayHost(host), strconv.Itoa(bound)), token)
	if bound != port && port != 0 {
		_, _ = fmt.Fprintf(out, "port %d is in use; listening on %d instead\n", port, bound)
	}
	_, _ = fmt.Fprintf(out, "probaci web UI: %s\n", url)
	if o.allowRemote {
		_, _ = fmt.Fprintln(errOut, "warning: --allow-remote serves plain HTTP; put a TLS proxy in front, or use an SSH tunnel")
	}
	if o.readOnly {
		_, _ = fmt.Fprintln(out, "read-only: runs cannot be started or canceled")
	}
	_, _ = fmt.Fprintln(out, "press Ctrl-C to stop")
	if browserAllowed(os.Getenv, isatty.IsTerminal(os.Stdout.Fd()), runtime.GOOS) && !o.noOpen && !g.jsonOut {
		if err := serveBrowser(url); err != nil {
			_, _ = fmt.Fprintf(errOut, "could not open a browser (%s); open the address above yourself\n", err)
		}
	}
	if err := srv.Serve(ctx, ln); err != nil {
		return failure(err)
	}
	return nil
}

// splitAddr accepts "host" or "host:port".
func splitAddr(addr string) (host string, port int, hasPort bool, err error) {
	h, p, splitErr := net.SplitHostPort(addr)
	if splitErr != nil {
		if addr == "" {
			return "", 0, false, errors.New("empty address")
		}
		return addr, 0, false, nil
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		return "", 0, false, fmt.Errorf("port %q is not a number", p)
	}
	return h, n, true, nil
}

// displayHost is the host to print: a wildcard bind is reachable here as
// 127.0.0.1, and that is one of the Host headers the server accepts.
func displayHost(host string) string {
	switch host {
	case "", "0.0.0.0", "::":
		return "127.0.0.1"
	}
	return host
}

// browserAllowed says whether serve may open a browser: a person at a terminal
// on a desktop, and nobody asked it not to.
func browserAllowed(getenv func(string) string, stdoutTTY bool, goos string) bool {
	if getenv("CI") != "" || getenv("PROBACI_NO_BROWSER") == "1" || !stdoutTTY {
		return false
	}
	if goos == "linux" || goos == "freebsd" || goos == "openbsd" || goos == "netbsd" {
		return getenv("DISPLAY") != "" || getenv("WAYLAND_DISPLAY") != ""
	}
	return true
}

// openBrowser opens url with the platform's own opener, so there is no
// dependency to add.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// appRunner is the web UI's Runner over probaci's own engine: every run builds
// a fresh App, so it reads the configuration again and gets its own run ID and
// log file, exactly as `probaci run` does.
type appRunner struct {
	targets []string

	mu       sync.Mutex
	dockerAt time.Time
	docker   string
}

// dockerTTL is how long the container check is trusted; asking the daemon on
// every page load would be slow.
const dockerTTL = 30 * time.Second

func (a *appRunner) Overview(ctx context.Context) (web.Overview, error) {
	layout := paths.Resolve()
	base := a.targets[0]
	cfg, sources, err := config.Load(config.LoadOptions{
		SystemConfigPath: layout.SystemConfig,
		UserConfigPath:   layout.Config,
		ProjectDir:       base,
		ExplicitPath:     g.config,
	})
	if err != nil {
		return web.Overview{}, err
	}
	if g.noDocker {
		cfg.Docker.Enabled = config.BoolPtr(false)
	}
	if g.backend != "" {
		cfg.Docker.Backend = config.Backend(g.backend)
	}
	ov := web.Overview{Stages: cfg.EnabledStages(), Home: layout.Home, Docker: a.dockerStatus(ctx, cfg)}
	for _, s := range sources {
		ov.Sources = append(ov.Sources, describeSource(s, layout, base))
	}
	for _, t := range a.targets {
		info := web.RepoInfo{Path: t}
		for _, l := range detect.DetectProject(t).Languages {
			info.Languages = append(info.Languages, l.Name)
		}
		ov.Repos = append(ov.Repos, info)
	}
	return ov, nil
}

func describeSource(s config.Source, layout paths.Layout, base string) string {
	switch s {
	case config.SourceUser:
		return string(s) + ": " + layout.Config
	case config.SourceSystem:
		return string(s) + ": " + layout.SystemConfig
	case config.SourceProject:
		if g.config != "" {
			return string(s) + ": " + g.config
		}
		return string(s) + ": " + filepath.Join(base, "probaci.json")
	}
	return string(s)
}

func (a *appRunner) dockerStatus(ctx context.Context, cfg config.Config) string {
	if !cfg.Docker.IsEnabled() {
		return "disabled; container stages are skipped"
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.docker != "" && time.Since(a.dockerAt) < dockerTTL {
		return a.docker
	}
	b, err := docker.New(cfg.Docker, cfg.Security, nil)
	if err == nil {
		check, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = b.Available(check)
		cancel()
	}
	a.docker = "available"
	if err != nil {
		a.docker = err.Error()
	}
	a.dockerAt = time.Now()
	return a.docker
}

func (a *appRunner) Run(ctx context.Context, req web.RunRequest, obs stage.Observer) (result.Aggregate, error) {
	app, err := newApp(a.targets[0])
	if err != nil {
		return result.Aggregate{}, err
	}
	defer app.Close()
	opts := stage.RunOptions{
		Repos:    a.targets,
		Only:     req.Stages,
		Jobs:     req.Jobs,
		Platform: g.platform,
		RunOpts:  platform.RunOpts{FullImage: g.fullImage, AllowSocketMount: app.Cfg.Docker.SocketMountAllowed()},
	}
	app.Logger.Info("run started from the web UI", "run", app.RunID.Short, "repos", len(a.targets),
		"jobs", opts.Jobs, "stages", fmt.Sprint(app.Engine.StageList(opts.Only, nil)))
	agg := app.Engine.Run(ctx, opts, obs)
	for _, repo := range agg.Repos {
		for _, r := range repo.Results {
			app.Logger.Info("stage finished", "repo", repo.Path, "stage", r.Stage, "status", string(r.Status), "summary", r.Summary)
		}
	}
	app.Logger.Info("run finished", "run", app.RunID.Short, "failed", agg.Failed())
	return agg, nil
}
