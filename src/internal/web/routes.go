package web

import "net/http"

// routes registers every page and action. Under ReadOnly the routes that start
// or cancel a run are not registered at all, rather than registered and
// refused: a route that does not exist cannot be reached by a bug in a check.
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /assets/{hash}/{path...}", s.serveAsset)
	mux.HandleFunc("GET /favicon.ico", s.serveFavicon)
	mux.HandleFunc("GET /{$}", s.handleOverview)
	mux.HandleFunc("GET /runs", s.handleRuns)
	mux.HandleFunc("GET /runs/{id}", s.handleRun)
	mux.HandleFunc("GET /runs/{id}/events", s.handleEvents)
	mux.HandleFunc("GET /runs/{id}/report.json", s.handleReport)
	mux.HandleFunc("GET /settings", s.handleSettings)
	mux.HandleFunc("GET /about/licenses", s.handleLicenses)
	mux.HandleFunc("GET "+HealthPath, s.handleHealth)
	// Stopping the server starts nothing, so a read-only server keeps it.
	mux.HandleFunc("POST "+StopPath, s.handleStop)

	if !s.opts.ReadOnly {
		mux.HandleFunc("POST /runs", s.handleStart)
		mux.HandleFunc("POST /runs/{id}/cancel", s.handleCancel)
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if s.opts.ReadOnly && r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			s.renderError(w, r, http.StatusMethodNotAllowed, "Read-only server",
				"This server was started with --read-only, so it cannot start or cancel a run.")
			return
		}
		s.renderError(w, r, http.StatusNotFound, "Not found", "There is no page at this address.")
	})
	return mux
}
