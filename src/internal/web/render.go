package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/simtabi/probaci/src/internal/result"
)

// assetSet is the embedded assets with a content hash each, for
// fingerprinted, immutable URLs.
type assetSet struct {
	files  map[string][]byte // path below assets/
	hashes map[string]string // first 12 hex chars of sha256
}

// The webfont's two files under assets/.
const (
	webfontCSS  = "vendor/tabler-icons/tabler-icons.css"
	webfontFont = "vendor/tabler-icons/tabler-icons.woff2"
)

// faviconAsset is the logo mark every page links as its icon.
const faviconAsset = "favicon.svg"

func loadAssets() (*assetSet, error) {
	a := &assetSet{files: map[string][]byte{}, hashes: map[string]string{}}
	err := fs.WalkDir(assetFS, "assets", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := assetFS.ReadFile(p)
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(p, "assets/")
		sum := sha256.Sum256(body)
		a.files[name] = body
		a.hashes[name] = hex.EncodeToString(sum[:])[:12]
		return nil
	})
	if err != nil {
		return nil, err
	}
	// The webfont CSS names its font relative to itself, and every asset URL
	// carries its own fingerprint, so a relative URL would resolve under the
	// CSS's fingerprint and 404. Point it at the font's own URL, then
	// fingerprint the rewritten CSS.
	css, ok := a.files[webfontCSS]
	if !ok {
		return nil, fmt.Errorf("web: the icon webfont CSS is not embedded")
	}
	font, err := a.url(webfontFont)
	if err != nil {
		return nil, fmt.Errorf("web: the webfont CSS is embedded without its font: %w", err)
	}
	body := bytes.ReplaceAll(css, []byte(`url("tabler-icons.woff2")`), []byte(`url("`+font+`")`))
	sum := sha256.Sum256(body)
	a.files[webfontCSS] = body
	a.hashes[webfontCSS] = hex.EncodeToString(sum[:])[:12]
	if _, ok := a.files[faviconAsset]; !ok {
		return nil, fmt.Errorf("web: the favicon is not embedded")
	}
	return a, nil
}

// url returns the fingerprinted URL of an asset. Asking for one that does not
// exist fails template execution, which the page tests turn into a failure
// rather than a broken page.
func (a *assetSet) url(name string) (string, error) {
	h, ok := a.hashes[name]
	if !ok {
		return "", fmt.Errorf("no embedded asset %q", name)
	}
	return "/assets/" + h + "/" + name, nil
}

// pageSet is every page template, each parsed once over the shared layout.
type pageSet struct {
	pages map[string]*template.Template
}

// pageNames lists the page templates: every templates/*.html but the layout.
func pageNames() ([]string, error) {
	entries, err := fs.ReadDir(templateFS, "templates")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || e.Name() == "layout.html" || !strings.HasSuffix(e.Name(), ".html") {
			continue
		}
		out = append(out, strings.TrimSuffix(e.Name(), ".html"))
	}
	sort.Strings(out)
	return out, nil
}

func funcs(a *assetSet) template.FuncMap {
	return template.FuncMap{
		"asset": a.url,
		"icon":  iconHTML,
		"iconc": iconClassHTML,
		"when":  func(t time.Time) string { return t.Local().Format("2006-01-02 15:04:05") },
		"iso":   func(t time.Time) string { return t.UTC().Format(time.RFC3339) },
		"clock": func(t time.Time) string { return t.Local().Format("15:04:05") },
		"dur": func(d time.Duration) string {
			if d <= 0 {
				return "–"
			}
			if d < time.Millisecond {
				return "<1ms"
			}
			if d < time.Second {
				return d.Round(time.Millisecond).String()
			}
			return d.Round(100 * time.Millisecond).String()
		},
		"base": filepath.Base,
		"stageClass": func(s result.Status) string {
			switch s {
			case result.StatusPass:
				return "badge-ok"
			case result.StatusSkip, result.StatusPending:
				return "badge-muted"
			case result.StatusRunning:
				return "badge-info"
			}
			return "badge-bad"
		},
		"runClass": func(s RunStatus) string {
			switch s {
			case RunPassed:
				return "badge-ok"
			case RunRunning:
				return "badge-info"
			case RunCanceled:
				return "badge-muted"
			}
			return "badge-bad"
		},
		"runAlert": func(s RunStatus) string {
			switch s {
			case RunPassed:
				return "alert-success"
			case RunRunning:
				return "alert-info"
			case RunCanceled:
				return "alert-secondary"
			}
			return "alert-danger"
		},
		"runText": func(s RunStatus) string {
			switch s {
			case RunPassed:
				return "Passed"
			case RunRunning:
				return "Running"
			case RunCanceled:
				return "Canceled"
			case RunFailed:
				return "Failed"
			}
			return "Could not run"
		},
		"orDash": func(s string) string {
			if s == "" {
				return "–"
			}
			return s
		},
		"join": strings.Join,
		"navItem": func(v view, key, href, icon, label string) navItem {
			return navItem{Active: v.Nav == key, Href: href, Icon: icon, Label: label}
		},
	}
}

// iconHTML renders a Tabler icon from the vendored Tabler Icons webfont. Icons
// are decorative here: every one sits beside text that says the same thing.
func iconHTML(name string) template.HTML {
	if !validIcon(name) {
		return ""
	}
	return template.HTML(`<i class="icon ti ti-` + name + `" aria-hidden="true"></i>`) // #nosec G203 -- name is [a-z0-9-] only
}

// iconClassHTML is iconHTML with extra classes ("icon-lg text-danger mb-2"),
// checked to the class-name alphabet.
func iconClassHTML(name, extra string) template.HTML {
	if !validIcon(name) {
		return ""
	}
	for _, r := range extra {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' && r != ' ' {
			return iconHTML(name)
		}
	}
	return template.HTML(`<i class="icon ti ti-` + name + ` ` + extra + `" aria-hidden="true"></i>`) // #nosec G203 -- name and extra are [a-z0-9- ] only
}

func validIcon(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

func parsePages(a *assetSet) (*pageSet, error) {
	base, err := template.New("layout.html").Funcs(funcs(a)).ParseFS(templateFS, "templates/layout.html")
	if err != nil {
		return nil, err
	}
	names, err := pageNames()
	if err != nil {
		return nil, err
	}
	ps := &pageSet{pages: map[string]*template.Template{}}
	for _, n := range names {
		t, err := base.Clone()
		if err != nil {
			return nil, err
		}
		if _, err := t.ParseFS(templateFS, path.Join("templates", n+".html")); err != nil {
			return nil, err
		}
		ps.pages[n] = t
	}
	return ps, nil
}

type navItem struct {
	Active            bool
	Href, Icon, Label string
}

// view is what every page template receives.
type view struct {
	Title    string
	Nav      string
	Authed   bool
	CSRF     string
	Version  string
	ReadOnly bool
	// Running is the run in progress, shown in the header on every page.
	Running *Run
	Data    any
}

func (s *Server) newView(r *http.Request, nav, title string, data any) view {
	v := view{Title: title, Nav: nav, Version: s.opts.Version, ReadOnly: s.opts.ReadOnly, Data: data}
	if sess := sessionFrom(r.Context()); sess != nil {
		v.Authed = true
		v.CSRF = sess.csrf
		if run, ok := s.runs.running(); ok {
			v.Running = &run
		}
	}
	return v
}

// render executes a page into a buffer first, so a template error becomes a
// clean 500 instead of half a page.
func (s *Server) render(w http.ResponseWriter, status int, page string, v view) {
	t, ok := s.pages.pages[page]
	if !ok {
		http.Error(w, "no such page template: "+page, http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", v); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

type errorData struct {
	Status         int
	Heading        string
	Message        string
	Link, LinkText string
}

func (s *Server) renderError(w http.ResponseWriter, r *http.Request, status int, heading, message string) {
	s.render(w, status, "error", s.newView(r, "", heading, errorData{Status: status, Heading: heading, Message: message}))
}

// renderErrorLink is renderError with a way forward, e.g. to the run in
// progress.
func (s *Server) renderErrorLink(w http.ResponseWriter, r *http.Request, status int, heading, message, link, text string) {
	s.render(w, status, "error", s.newView(r, "", heading,
		errorData{Status: status, Heading: heading, Message: message, Link: link, LinkText: text}))
}

func (s *Server) renderUnauthorized(w http.ResponseWriter, r *http.Request) {
	s.renderError(w, r, http.StatusUnauthorized, "Sign-in link required",
		"Open the address probaci serve printed when it started. It carries a one-time token that signs this browser in.")
}

// serveAsset answers /assets/{hash}/{path...}. A stale hash is a 404, not the
// current file: an immutable URL must never change content.
func (s *Server) serveAsset(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("path")
	body, ok := s.assets.files[name]
	if !ok || r.PathValue("hash") != s.assets.hashes[name] {
		http.NotFound(w, r)
		return
	}
	switch path.Ext(name) {
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".woff2":
		w.Header().Set("Content-Type", "font/woff2")
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(body)
}

// serveFavicon answers /favicon.ico, which browsers ask for on their own, with
// the same SVG the pages link.
func (s *Server) serveFavicon(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(s.assets.files[faviconAsset])
}
