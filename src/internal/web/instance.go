package web

import (
	"net/http"
	"sync"
)

// The contract a script or a second launch relies on.
const (
	// HealthPath answers 200 and HealthBody to a request carrying the launch
	// token in HealthTokenHeader, and 401 to anything else.
	HealthPath        = "/healthz"
	HealthTokenHeader = "X-Probaci-Token" // #nosec G101 -- a header name, not a credential
	HealthBody        = "probaci"

	// StopPath is the "Stop server" action: a same-origin POST with the
	// session's CSRF token, which shuts the server down gracefully.
	StopPath = "/server/stop"
)

// stopper is closed once, by Stop.
type stopper struct {
	once sync.Once
	ch   chan struct{}
}

// Stop asks Serve to shut down, as Ctrl-C does. Safe to call more than once.
func (s *Server) Stop() { s.stop.once.Do(func() { close(s.stop.ch) }) }

// Stopped is closed once Stop has been called.
func (s *Server) Stopped() <-chan struct{} { return s.stop.ch }

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(HealthBody + "\n"))
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	s.render(w, http.StatusOK, "error", s.newView(r, "", "Server stopped", errorData{
		Status: http.StatusOK, Heading: "Server stopped",
		Message: "probaci has shut down. You can close this tab; run probaci serve again to reopen the UI.",
	}))
	s.Stop()
}
