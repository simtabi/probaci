package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/simtabi/probaci/src/internal/result"
	"github.com/simtabi/probaci/src/internal/stage"
)

// RunStatus is where a run started from the UI stands.
type RunStatus string

const (
	RunRunning  RunStatus = "running"
	RunPassed   RunStatus = "passed"
	RunFailed   RunStatus = "failed"
	RunCanceled RunStatus = "canceled"
	RunErrored  RunStatus = "errored"
)

// maxLines bounds the output a run keeps in memory and on disk. Lines past it
// are counted, not kept; the full output is still in the run's log file.
const maxLines = 5000

// maxHistory is how many finished runs the Runs page lists.
const maxHistory = 100

// StageState is one stage of one repository, as the run page shows it.
type StageState struct {
	Name     string        `json:"name"`
	Status   result.Status `json:"status"`
	Summary  string        `json:"summary,omitempty"`
	Output   string        `json:"output,omitempty"`
	Err      string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration_ns,omitempty"`
}

// RepoState is one repository of a run.
type RepoState struct {
	Path   string       `json:"path"`
	Stages []StageState `json:"stages"`
}

// LogLine is one line of streamed stage output, already redacted by the engine.
type LogLine struct {
	Seq   int       `json:"seq"`
	Time  time.Time `json:"time"`
	Repo  string    `json:"repo"`
	Stage string    `json:"stage"`
	Text  string    `json:"text"`
}

// Run is one pipeline run started from the UI.
type Run struct {
	ID       string      `json:"id"`
	Started  time.Time   `json:"started"`
	Finished time.Time   `json:"finished,omitzero"`
	Status   RunStatus   `json:"status"`
	Err      string      `json:"error,omitempty"`
	Request  RunRequest  `json:"request"`
	Repos    []RepoState `json:"repos"`
	Lines    []LogLine   `json:"lines"`
	Dropped  int         `json:"dropped_lines,omitempty"`

	seq     int
	cancel  context.CancelFunc
	changed chan struct{} // closed and replaced on every change
}

// Duration is how long the run took, or has taken so far.
func (r Run) Duration(now time.Time) time.Duration {
	end := r.Finished
	if end.IsZero() {
		end = now
	}
	return end.Sub(r.Started).Round(time.Second)
}

// Counts tallies the stage outcomes across every repository.
func (r Run) Counts() map[result.Status]int {
	out := map[result.Status]int{}
	for _, repo := range r.Repos {
		for _, st := range repo.Stages {
			out[st.Status]++
		}
	}
	return out
}

// registry holds the runs: at most one in progress, the rest finished.
type registry struct {
	mu     sync.Mutex
	byID   map[string]*Run
	order  []string // newest first
	active *Run
	dir    string
	now    func() time.Time
	wg     sync.WaitGroup
}

// errBusy is returned when a run is already in progress.
var errBusy = errors.New("a run is already in progress")

func newRegistry(dir string, now func() time.Time) *registry {
	g := &registry{byID: map[string]*Run{}, dir: dir, now: now}
	g.load()
	return g
}

// load reads finished runs from the history directory, newest first. A file
// that does not parse is skipped rather than failing startup.
func (g *registry) load() {
	if g.dir == "" {
		return
	}
	entries, err := os.ReadDir(g.dir)
	if err != nil {
		return
	}
	var runs []*Run
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(g.dir, e.Name())) // #nosec G304 -- our own state directory
		if err != nil {
			continue
		}
		var r Run
		if json.Unmarshal(body, &r) != nil || !validRunID(r.ID) {
			continue
		}
		if r.Status == RunRunning {
			// The server stopped while it ran; nothing will finish it now.
			r.Status, r.Err = RunCanceled, "the server stopped before the run finished"
		}
		runs = append(runs, &r)
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].Started.After(runs[j].Started) })
	if len(runs) > maxHistory {
		runs = runs[:maxHistory]
	}
	for _, r := range runs {
		g.byID[r.ID] = r
		g.order = append(g.order, r.ID)
	}
}

// newRunID is sortable by time and unguessable enough to name a file.
func newRunID(now time.Time) string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return now.UTC().Format("20060102-150405") + "-" + hex.EncodeToString(b)
}

// validRunID accepts only what newRunID makes, so an ID from a URL can never
// name a path outside the history directory.
func validRunID(id string) bool {
	if len(id) != 22 {
		return false
	}
	for i, c := range id {
		switch {
		case i == 8 || i == 15:
			if c != '-' {
				return false
			}
		case c >= '0' && c <= '9', i > 15 && c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}

// start begins a run in the background, or returns errBusy and the run in
// progress.
func (g *registry) start(base context.Context, rn Runner, req RunRequest, plan Overview) (*Run, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.active != nil {
		return g.active, errBusy
	}
	ctx, cancel := context.WithCancel(base)
	r := &Run{
		ID:      newRunID(g.now()),
		Started: g.now(),
		Status:  RunRunning,
		Request: req,
		cancel:  cancel,
		changed: make(chan struct{}),
	}
	stages := req.Stages
	if len(stages) == 0 {
		stages = plan.Stages
	}
	for _, repo := range plan.Repos {
		rs := RepoState{Path: repo.Path}
		for _, name := range stages {
			rs.Stages = append(rs.Stages, StageState{Name: name, Status: result.StatusPending})
		}
		r.Repos = append(r.Repos, rs)
	}
	g.byID[r.ID] = r
	g.order = append([]string{r.ID}, g.order...)
	g.active = r

	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		defer cancel()
		agg, err := rn.Run(ctx, req, func(e stage.Event) { g.observe(r, e) })
		g.finish(r, agg, err, ctx.Err() != nil)
	}()
	return r, nil
}

// observe folds one engine event into the run.
func (g *registry) observe(r *Run, e stage.Event) {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch {
	case e.Line != "":
		if len(r.Lines) >= maxLines {
			r.Dropped++
			break
		}
		r.seq++
		r.Lines = append(r.Lines, LogLine{Seq: r.seq, Time: g.now(), Repo: e.Repo, Stage: e.Stage, Text: e.Line})
	case e.Result != nil:
		st := stageOf(r, e.Repo, e.Stage)
		st.Status = e.Result.Status
		st.Summary, st.Output, st.Err, st.Duration = e.Result.Summary, e.Result.Output, e.Result.Err, e.Result.Duration
	default:
		stageOf(r, e.Repo, e.Stage).Status = e.Status
	}
	g.notify(r)
}

// stageOf returns the run's state for a repository's stage, adding either when
// the plan did not list it (a platform stage resolved at run time).
func stageOf(r *Run, repo, name string) *StageState {
	ri := -1
	for i := range r.Repos {
		if r.Repos[i].Path == repo {
			ri = i
			break
		}
	}
	if ri < 0 {
		r.Repos = append(r.Repos, RepoState{Path: repo})
		ri = len(r.Repos) - 1
	}
	rs := &r.Repos[ri]
	for i := range rs.Stages {
		if rs.Stages[i].Name == name {
			return &rs.Stages[i]
		}
	}
	rs.Stages = append(rs.Stages, StageState{Name: name, Status: result.StatusPending})
	return &rs.Stages[len(rs.Stages)-1]
}

// finish records the run's outcome, takes the final results from the
// aggregate, and saves it.
func (g *registry) finish(r *Run, agg result.Aggregate, err error, canceled bool) {
	g.mu.Lock()
	for _, repo := range agg.Repos {
		for _, res := range repo.Results {
			st := stageOf(r, repo.Path, res.Stage)
			st.Status, st.Summary, st.Output, st.Err, st.Duration = res.Status, res.Summary, res.Output, res.Err, res.Duration
		}
	}
	r.Finished = g.now()
	switch {
	case canceled:
		r.Status = RunCanceled
	case err != nil:
		r.Status, r.Err = RunErrored, err.Error()
	case agg.Failed():
		r.Status = RunFailed
	default:
		r.Status = RunPassed
	}
	// A stage the run never reached stays pending; say so rather than leave
	// it looking queued.
	for i := range r.Repos {
		for j := range r.Repos[i].Stages {
			if st := &r.Repos[i].Stages[j]; st.Status == result.StatusPending || st.Status == result.StatusRunning {
				st.Status = result.StatusSkip
				if st.Summary == "" {
					st.Summary = "not run"
				}
			}
		}
	}
	if g.active == r {
		g.active = nil
	}
	g.notify(r)
	snapshot, _ := json.MarshalIndent(r, "", "  ")
	g.mu.Unlock()
	g.save(r.ID, snapshot)
}

func (g *registry) save(id string, body []byte) {
	if g.dir == "" || len(body) == 0 {
		return
	}
	if err := os.MkdirAll(g.dir, 0o700); err != nil {
		return
	}
	tmp := filepath.Join(g.dir, id+".json.tmp")
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, filepath.Join(g.dir, id+".json"))
}

// notify wakes every event stream waiting on r. Callers hold g.mu.
func (g *registry) notify(r *Run) {
	if r.changed != nil {
		close(r.changed)
	}
	r.changed = make(chan struct{})
}

// cancelRun stops a run in progress. It reports whether there was one to stop.
func (g *registry) cancelRun(id string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	r := g.byID[id]
	if r == nil || r.Status != RunRunning || r.cancel == nil {
		return false
	}
	r.cancel()
	return true
}

// get returns a copy of a run, safe to render while the run goes on, and the
// channel that closes on its next change.
func (g *registry) get(id string) (Run, <-chan struct{}, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	r := g.byID[id]
	if r == nil {
		return Run{}, nil, false
	}
	return copyRun(r), r.changed, true
}

// list returns copies of the runs, newest first, without their output.
func (g *registry) list() []Run {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]Run, 0, len(g.order))
	for _, id := range g.order {
		c := copyRun(g.byID[id])
		c.Lines = nil
		out = append(out, c)
	}
	return out
}

// running returns the run in progress, if any.
func (g *registry) running() (Run, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.active == nil {
		return Run{}, false
	}
	return copyRun(g.active), true
}

func copyRun(r *Run) Run {
	c := *r
	c.cancel, c.changed = nil, nil
	c.Repos = make([]RepoState, len(r.Repos))
	for i, repo := range r.Repos {
		c.Repos[i] = RepoState{Path: repo.Path, Stages: append([]StageState(nil), repo.Stages...)}
	}
	c.Lines = append([]LogLine(nil), r.Lines...)
	return c
}

// wait blocks until no run is in progress.
func (g *registry) wait() { g.wg.Wait() }
