// Package web is the `serve` command's localhost web UI.
//
// Server-rendered html/template pages over the same stage engine the CLI and
// the TUI drive, with a little vanilla JavaScript and Tabler's CSS, all
// embedded in the binary: the page makes no network request beyond this
// server. It never imports internal/cli, which holds process-wide flag state a
// long-lived server cannot share; the CLI hands it a Runner instead.
//
// The shape follows the other Simtabi Go web UIs (ichava/icon-sets-sync,
// simtabi/authama): a one-time sign-in token exchanged for a session cookie,
// a Host allowlist, same-origin and CSRF checks on every change, a strict
// Content-Security-Policy, fingerprinted embedded assets, and a --read-only
// mode that does not register the routes that act.
package web

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/simtabi/probaci/src/internal/result"
	"github.com/simtabi/probaci/src/internal/stage"
)

// Runner is everything the UI needs from probaci. The CLI implements it over
// pkg/probaci; tests implement it with a fake.
type Runner interface {
	// Overview describes the targets and the configured pipeline. It runs
	// nothing.
	Overview(ctx context.Context) (Overview, error)

	// Run runs the pipeline over every target, sending each engine event to
	// obs. Canceling ctx stops it.
	Run(ctx context.Context, req RunRequest, obs stage.Observer) (result.Aggregate, error)
}

// RunRequest is what the run form asks for.
type RunRequest struct {
	// Stages to run, in pipeline order. Empty runs every enabled stage.
	Stages []string
	// Jobs is how many repositories run at once; below 1 means 1.
	Jobs int
}

// Overview is the dashboard's picture of what a run would cover.
type Overview struct {
	Repos   []RepoInfo
	Stages  []string // enabled stages, in pipeline order
	Docker  string   // "available", or why containers cannot run
	Sources []string // config files that contributed, lowest precedence first
	Home    string   // probaci's configuration home
}

// RepoInfo is one target repository.
type RepoInfo struct {
	Path      string
	Languages []string
}

// DefaultHistoryDir is where finished runs are kept: XDG state on every
// platform but Windows, per the house layout ($XDG_STATE_HOME, else
// ~/.local/state; %LocalAppData% on Windows).
func DefaultHistoryDir(lookup func(string) (string, bool)) string {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	base, _ := lookup("XDG_STATE_HOME")
	if base == "" && runtime.GOOS == "windows" {
		base, _ = lookup("LOCALAPPDATA")
	}
	if base == "" {
		home, _ := lookup("HOME")
		if home == "" {
			home, _ = os.UserHomeDir()
		}
		if home == "" {
			return ""
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "probaci", "runs")
}

// Options configures a Server.
type Options struct {
	// Runner runs the pipeline. Required.
	Runner Runner

	// Token is the launch token printed in the URL. Required.
	Token string

	// AllowedHosts are the Host header values answered (see AllowedHosts).
	AllowedHosts []string

	// AllowRemote records that the bind is not loopback; it lets a TLS
	// reverse proxy's X-Forwarded-Proto mark the session cookie Secure.
	AllowRemote bool

	// ReadOnly registers no route that starts or cancels a run.
	ReadOnly bool

	// Version and BindAddr are shown on the settings page.
	Version  string
	BindAddr string

	// HistoryDir keeps finished runs, one JSON file each. Empty keeps them in
	// memory only. See DefaultHistoryDir.
	HistoryDir string

	// Now is the clock. Nil means time.Now.
	Now func() time.Time
}

// Server is the web UI.
type Server struct {
	opts     Options
	pages    *pageSet
	assets   *assetSet
	sessions sessions
	handler  http.Handler
	runs     *registry

	// stop is closed by the "Stop server" action.
	stop stopper

	// base is canceled on shutdown, stopping a run in progress.
	base   context.Context
	cancel context.CancelFunc
}

// New builds a Server. It parses every template once, so a template error is
// a startup error rather than a 500 on first visit.
func New(opts Options) (*Server, error) {
	if opts.Runner == nil {
		return nil, errors.New("web: a Runner is required")
	}
	if opts.Token == "" {
		return nil, errors.New("web: a Token is required")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	assets, err := loadAssets()
	if err != nil {
		return nil, err
	}
	pages, err := parsePages(assets)
	if err != nil {
		return nil, err
	}
	s := &Server{opts: opts, pages: pages, assets: assets, stop: stopper{ch: make(chan struct{})}}
	s.base, s.cancel = context.WithCancel(context.Background())
	s.runs = newRegistry(opts.HistoryDir, opts.Now)
	s.handler = securityHeaders(s.hostCheck(s.auth(s.sameOrigin(s.routes()))))
	return s, nil
}

// Handler returns the full middleware chain and routes.
func (s *Server) Handler() http.Handler { return s.handler }

func (s *Server) now() time.Time { return s.opts.Now() }

// Serve answers on ln until ctx is canceled or Stop is called, then shuts
// down with a ten-second grace for requests in flight.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	srv := &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()

	select {
	case err := <-errc:
		s.cancel()
		return err
	case <-ctx.Done():
	case <-s.stop.ch:
	}
	// Cancel the run first: an open event stream only ends when its run
	// does, so shutting the server down before it would wait out the grace.
	s.cancel()
	shutdown, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	if err := srv.Shutdown(shutdown); err != nil {
		return err
	}
	s.runs.wait()
	if err := <-errc; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Close stops background work without serving; for tests.
func (s *Server) Close() {
	s.cancel()
	s.runs.wait()
}

type ctxKey int

const sessionKey ctxKey = 0

func withSession(ctx context.Context, s *session) context.Context {
	return context.WithValue(ctx, sessionKey, s)
}

func sessionFrom(ctx context.Context) *session {
	s, _ := ctx.Value(sessionKey).(*session)
	return s
}
