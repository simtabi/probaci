package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/simtabi/probaci/src/internal/result"
	"github.com/simtabi/probaci/src/internal/stage"
)

const testHost = "127.0.0.1:7844"

// fakeRunner stands in for probaci's engine. By default a run emits a start,
// one line and a pass for every requested stage of every repository.
type fakeRunner struct {
	mu       sync.Mutex
	overview Overview
	calls    []RunRequest
	// block, when set, makes Run wait until ctx is canceled.
	block bool
	// fail marks this stage as failed.
	fail string
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{overview: Overview{
		Repos:   []RepoInfo{{Path: "/work/alpha", Languages: []string{"go"}}, {Path: "/work/beta"}},
		Stages:  []string{"detect", "lint", "test"},
		Docker:  "available",
		Sources: []string{"/home/u/.config/probaci/config.json"},
		Home:    "/home/u/.config/probaci",
	}}
}

func (f *fakeRunner) Overview(context.Context) (Overview, error) { return f.overview, nil }

func (f *fakeRunner) Run(ctx context.Context, req RunRequest, obs stage.Observer) (result.Aggregate, error) {
	f.mu.Lock()
	f.calls = append(f.calls, req)
	block, fail := f.block, f.fail
	f.mu.Unlock()
	var agg result.Aggregate
	for _, repo := range f.overview.Repos {
		rep := result.RepoReport{Path: repo.Path}
		for _, name := range req.Stages {
			obs(stage.Event{Repo: repo.Path, Stage: name, Status: result.StatusRunning})
			obs(stage.Event{Repo: repo.Path, Stage: name, Status: result.StatusRunning, Line: "output of " + name})
			if block {
				<-ctx.Done()
				return agg, nil
			}
			res := result.Result{Stage: name, Status: result.StatusPass, Summary: name + " ok", Duration: time.Second}
			if name == fail {
				res.Status, res.Summary = result.StatusFail, name+" failed"
			}
			obs(stage.Event{Repo: repo.Path, Stage: name, Status: res.Status, Result: &res})
			rep.Results = append(rep.Results, res)
		}
		agg.Repos = append(agg.Repos, rep)
	}
	return agg, nil
}

func newTestServerWith(t *testing.T, opts Options) *Server {
	t.Helper()
	if opts.Runner == nil {
		opts.Runner = newFakeRunner()
	}
	if opts.Token == "" {
		opts.Token = "launch-token"
	}
	opts.AllowedHosts = AllowedHosts("127.0.0.1", 7844, nil)
	opts.Version = "test"
	s, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func newTestServer(t *testing.T) (*Server, *fakeRunner) {
	t.Helper()
	f := newFakeRunner()
	return newTestServerWith(t, Options{Runner: f}), f
}

// do sends a request with the right Host and returns the recorded response.
func do(s *Server, req *http.Request) *httptest.ResponseRecorder {
	if req.Host == "" || req.Host == "example.com" {
		req.Host = testHost
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

// client is a signed-in browser: a session cookie and the page's CSRF token.
type client struct {
	t      *testing.T
	s      *Server
	cookie *http.Cookie
	csrf   string
}

var csrfMeta = regexp.MustCompile(`<meta name="csrf-token" content="([^"]+)">`)

func signIn(t *testing.T, s *Server) *client {
	t.Helper()
	rec := do(s, httptest.NewRequest(http.MethodGet, "/?token="+s.opts.Token, nil))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("sign-in: status %d", rec.Code)
	}
	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookie {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("sign-in set no session cookie")
	}
	c := &client{t: t, s: s, cookie: cookie}
	m := csrfMeta.FindStringSubmatch(c.get("/").Body.String())
	if m == nil {
		t.Fatal("no CSRF token on the overview")
	}
	c.csrf = m[1]
	return c
}

func (c *client) get(path string) *httptest.ResponseRecorder {
	c.t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(c.cookie)
	return do(c.s, req)
}

// post sends a same-origin form POST carrying the CSRF token.
func (c *client) post(path string, form url.Values) *httptest.ResponseRecorder {
	c.t.Helper()
	if form == nil {
		form = url.Values{}
	}
	form.Set(csrfField, c.csrf)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://"+testHost)
	req.AddCookie(c.cookie)
	return do(c.s, req)
}

// startRun posts the run form and returns the new run's ID.
func (c *client) startRun(stages ...string) string {
	c.t.Helper()
	rec := c.post("/runs", url.Values{"stage": stages})
	if rec.Code != http.StatusSeeOther {
		c.t.Fatalf("start run: status %d: %s", rec.Code, rec.Body.String())
	}
	id := strings.TrimPrefix(rec.Header().Get("Location"), "/runs/")
	if !validRunID(id) {
		c.t.Fatalf("start run redirected to %q", rec.Header().Get("Location"))
	}
	return id
}

// waitDone waits for the run to finish and returns it.
func waitDone(t *testing.T, s *Server, id string) Run {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r, _, ok := s.runs.get(id)
		if ok && r.Status != RunRunning {
			return r
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("run %s did not finish", id)
	return Run{}
}
