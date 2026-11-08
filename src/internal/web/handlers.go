package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

// overviewTimeout bounds a dashboard's look at the targets: detection reads a
// few files, and the container check asks the daemon once.
const overviewTimeout = 10 * time.Second

type overviewData struct {
	Overview Overview
	Err      string
	Recent   []Run
	MaxJobs  int
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), overviewTimeout)
	defer cancel()
	ov, err := s.opts.Runner.Overview(ctx)
	d := overviewData{Overview: ov, MaxJobs: max(1, len(ov.Repos))}
	if err != nil {
		d.Err = err.Error()
	}
	d.Recent = s.runs.list()
	if len(d.Recent) > 5 {
		d.Recent = d.Recent[:5]
	}
	s.render(w, http.StatusOK, "overview", s.newView(r, "overview", "Overview", d))
}

// handleStart starts a run with the stages the form ticked.
func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderError(w, r, http.StatusBadRequest, "Bad form", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), overviewTimeout)
	ov, err := s.opts.Runner.Overview(ctx)
	cancel()
	if err != nil {
		s.renderError(w, r, http.StatusInternalServerError, "Cannot read the configuration", err.Error())
		return
	}
	req := RunRequest{Jobs: 1}
	// Keep only stages the configuration enables, in pipeline order, whatever
	// order or names the form sent.
	picked := r.PostForm["stage"]
	for _, name := range ov.Stages {
		if slices.Contains(picked, name) {
			req.Stages = append(req.Stages, name)
		}
	}
	if len(req.Stages) == 0 {
		s.renderErrorLink(w, r, http.StatusBadRequest, "No stages chosen",
			"Tick at least one stage to run.", "/", "Back to the overview")
		return
	}
	if n, err := strconv.Atoi(r.PostFormValue("jobs")); err == nil && n > 1 {
		req.Jobs = min(n, max(1, len(ov.Repos)))
	}
	run, err := s.runs.start(s.base, s.opts.Runner, req, ov)
	if errors.Is(err, errBusy) {
		s.renderErrorLink(w, r, http.StatusConflict, "A run is already in progress",
			"probaci runs one pipeline at a time. Wait for it to finish, or cancel it.", "/runs/"+run.ID, "Open the run in progress")
		return
	}
	http.Redirect(w, r, "/runs/"+run.ID, http.StatusSeeOther)
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	s.render(w, http.StatusOK, "runs", s.newView(r, "runs", "Runs", s.runs.list()))
}

type runData struct {
	Run    Run
	Now    time.Time
	LastID int
}

func (s *Server) lookupRun(w http.ResponseWriter, r *http.Request) (Run, <-chan struct{}, bool) {
	id := r.PathValue("id")
	if !validRunID(id) {
		s.renderError(w, r, http.StatusNotFound, "No such run", "There is no run with this ID.")
		return Run{}, nil, false
	}
	run, changed, ok := s.runs.get(id)
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "No such run", "There is no run with this ID.")
		return Run{}, nil, false
	}
	return run, changed, true
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	run, _, ok := s.lookupRun(w, r)
	if !ok {
		return
	}
	last := 0
	if n := len(run.Lines); n > 0 {
		last = run.Lines[n-1].Seq
	}
	s.render(w, http.StatusOK, "run", s.newView(r, "runs", "Run "+run.ID, runData{Run: run, Now: s.now(), LastID: last}))
}

func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	run, _, ok := s.lookupRun(w, r)
	if !ok {
		return
	}
	s.runs.cancelRun(run.ID)
	http.Redirect(w, r, "/runs/"+run.ID, http.StatusSeeOther)
}

// handleReport serves the run's results as JSON, in the shape `probaci run
// --json` prints plus the run's own status.
func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	run, _, ok := s.lookupRun(w, r)
	if !ok {
		return
	}
	run.Lines = nil
	body, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="probaci-`+run.ID+`.json"`)
	_, _ = w.Write(body)
}

// streamWait is how long an event stream idles before a keep-alive comment.
const streamWait = 15 * time.Second

// handleEvents streams a run's progress as server-sent events: "line" for each
// line of output after ?after=, "stage" for each stage's current state, and
// "done" once the run has finished.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	run, changed, ok := s.lookupRun(w, r)
	if !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	after, _ := strconv.Atoi(r.URL.Query().Get("after"))
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)

	sent := map[string]string{} // repo|stage -> status|summary last sent
	for {
		for _, l := range run.Lines {
			if l.Seq > after {
				writeEvent(w, "line", l.Seq, l)
				after = l.Seq
			}
		}
		for _, repo := range run.Repos {
			for _, st := range repo.Stages {
				key, val := repo.Path+"|"+st.Name, string(st.Status)+"|"+st.Summary
				if sent[key] != val {
					sent[key] = val
					writeEvent(w, "stage", 0, map[string]any{
						"repo": repo.Path, "stage": st.Name, "status": st.Status,
						"summary": st.Summary, "duration": st.Duration.Round(100 * time.Millisecond).String(),
					})
				}
			}
		}
		if run.Status != RunRunning {
			writeEvent(w, "done", 0, map[string]any{"status": run.Status, "error": run.Err})
			flusher.Flush()
			return
		}
		flusher.Flush()
		select {
		case <-r.Context().Done():
			return
		case <-s.base.Done():
			return
		case <-changed:
		case <-time.After(streamWait):
			_, _ = fmt.Fprint(w, ": keep-alive\n\n")
		}
		run, changed, _ = s.runs.get(run.ID)
	}
}

func writeEvent(w http.ResponseWriter, event string, id int, data any) {
	body, err := json.Marshal(data)
	if err != nil {
		return
	}
	if id > 0 {
		_, _ = fmt.Fprintf(w, "id: %d\n", id)
	}
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, body)
}

type settingsData struct {
	Overview   Overview
	Err        string
	BindAddr   string
	HistoryDir string
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), overviewTimeout)
	defer cancel()
	ov, err := s.opts.Runner.Overview(ctx)
	d := settingsData{Overview: ov, BindAddr: s.opts.BindAddr, HistoryDir: s.opts.HistoryDir}
	if err != nil {
		d.Err = err.Error()
	}
	s.render(w, http.StatusOK, "settings", s.newView(r, "settings", "Settings", d))
}

// license is one vendored package's license, as the licenses page shows it.
type license struct {
	Package string
	File    string
	Text    string
}

// embeddedLicenses reads the license texts shipped with the vendored assets.
func (s *Server) embeddedLicenses() ([]license, error) {
	entries, err := fs.ReadDir(assetFS, "assets/vendor/LICENSES")
	if err != nil {
		return nil, err
	}
	var out []license
	for _, e := range entries {
		body, err := fs.ReadFile(assetFS, "assets/vendor/LICENSES/"+e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, license{Package: strings.TrimSuffix(e.Name(), ".txt"), File: e.Name(), Text: string(body)})
	}
	return out, nil
}

func (s *Server) handleLicenses(w http.ResponseWriter, r *http.Request) {
	ls, err := s.embeddedLicenses()
	if err != nil {
		s.renderError(w, r, http.StatusInternalServerError, "Licenses unavailable", err.Error())
		return
	}
	s.render(w, http.StatusOK, "licenses", s.newView(r, "", "Third-party licenses", ls))
}
