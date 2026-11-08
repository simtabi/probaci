package web

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/simtabi/probaci/src/internal/result"
)

// pages is every page a signed-in browser can open, with one run recorded so
// the run page exists. The count is asserted, so a page dropped from this
// list fails rather than going untested.
func pages(t *testing.T, c *client, id string) []string {
	t.Helper()
	ps := []string{"/", "/runs", "/runs/" + id, "/settings", "/about/licenses"}
	names, err := pageNames()
	if err != nil {
		t.Fatal(err)
	}
	// Every template but "error" has a route above.
	if len(ps) != len(names)-1 {
		t.Fatalf("%d pages listed, %d templates (%v): add the new page here", len(ps), len(names), names)
	}
	return ps
}

var faviconLink = regexp.MustCompile(`<link rel="icon" type="image/svg\+xml" href="(/assets/[0-9a-f]{12}/favicon\.svg)">`)

// Every page links the favicon as SVG, and the linked asset answers 200 as
// image/svg+xml: the house rule for every Go web UI.
func TestEveryPageLinksTheFavicon(t *testing.T) {
	s, _ := newTestServer(t)
	c := signIn(t, s)
	id := c.startRun("detect")
	waitDone(t, s, id)

	checked := 0
	for _, p := range append(pages(t, c, id), "/no-such-page") {
		rec := c.get(p)
		m := faviconLink.FindStringSubmatch(rec.Body.String())
		if m == nil {
			t.Errorf("%s (%d) has no SVG favicon link", p, rec.Code)
			continue
		}
		icon := c.get(m[1])
		if icon.Code != http.StatusOK || icon.Header().Get("Content-Type") != "image/svg+xml" {
			t.Errorf("%s: favicon answered %d %q", p, icon.Code, icon.Header().Get("Content-Type"))
		}
		checked++
	}
	if checked < 6 {
		t.Fatalf("checked %d pages, want at least 6", checked)
	}
	// The sign-in page a browser without a session sees links it too.
	if rec := do(s, httptest.NewRequest(http.MethodGet, "/", nil)); !faviconLink.MatchString(rec.Body.String()) {
		t.Error("the 401 page has no favicon link")
	}
}

// The favicon is the logo mark: a primary rounded square on a 32x32 box with
// a white glyph, and /favicon.ico serves it without a session.
func TestFaviconIsTheLogoMark(t *testing.T) {
	s, _ := newTestServer(t)
	svg := string(s.assets.files[faviconAsset])
	for _, want := range []string{`viewBox="0 0 32 32"`, `rx="7"`, `fill="#066fd1"`, `stroke="#fff"`} {
		if !strings.Contains(svg, want) {
			t.Errorf("favicon.svg lacks %s", want)
		}
	}
	rec := do(s, httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/svg+xml" || rec.Body.String() != svg {
		t.Errorf("/favicon.ico answered %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
}

func TestEveryPageRendersAndCarriesTheSecurityHeaders(t *testing.T) {
	s, _ := newTestServer(t)
	c := signIn(t, s)
	id := c.startRun("detect", "lint")
	waitDone(t, s, id)
	for _, p := range pages(t, c, id) {
		rec := c.get(p)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", p, rec.Code)
		}
		if rec.Header().Get("Content-Security-Policy") != ContentSecurityPolicy {
			t.Errorf("%s: no CSP", p)
		}
		if strings.Contains(rec.Body.String(), "<script>") || strings.Contains(rec.Body.String(), " style=") {
			t.Errorf("%s: inline script or style, which the CSP refuses", p)
		}
	}
}

func TestSignIn(t *testing.T) {
	s, _ := newTestServer(t)
	if rec := do(s, httptest.NewRequest(http.MethodGet, "/", nil)); rec.Code != http.StatusUnauthorized {
		t.Errorf("no session: %d, want 401", rec.Code)
	}
	if rec := do(s, httptest.NewRequest(http.MethodGet, "/?token=wrong", nil)); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong token: %d, want 401", rec.Code)
	}
	rec := do(s, httptest.NewRequest(http.MethodGet, "/runs?token="+s.opts.Token, nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/runs" {
		t.Errorf("good token: %d to %q, want 303 to /runs without the token", rec.Code, rec.Header().Get("Location"))
	}
	c := signIn(t, s)
	if rec := c.get("/"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "/work/alpha") {
		t.Errorf("signed in: %d", rec.Code)
	}
}

func TestHealthNeedsTheTokenInAHeader(t *testing.T) {
	s, _ := newTestServer(t)
	if rec := do(s, httptest.NewRequest(http.MethodGet, HealthPath, nil)); rec.Code != http.StatusUnauthorized {
		t.Errorf("no token: %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodGet, HealthPath, nil)
	req.Header.Set(HealthTokenHeader, s.opts.Token)
	if rec := do(s, req); rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != HealthBody {
		t.Errorf("with token: %d %q", rec.Code, rec.Body.String())
	}
}

func TestUnknownHostIsRefused(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/?token="+s.opts.Token, nil)
	req.Host = "evil.example:7844"
	if rec := do(s, req); rec.Code != http.StatusMisdirectedRequest {
		t.Errorf("rebinding host: %d, want 421", rec.Code)
	}
}

func TestChangesNeedSameOriginAndCSRF(t *testing.T) {
	s, f := newTestServer(t)
	c := signIn(t, s)

	noToken := httptest.NewRequest(http.MethodPost, "/runs", strings.NewReader("stage=detect"))
	noToken.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	noToken.Header.Set("Origin", "http://"+testHost)
	noToken.AddCookie(c.cookie)
	if rec := do(s, noToken); rec.Code != http.StatusForbidden {
		t.Errorf("no CSRF token: %d, want 403", rec.Code)
	}

	form := url.Values{"stage": {"detect"}, csrfField: {c.csrf}}
	cross := httptest.NewRequest(http.MethodPost, "/runs", strings.NewReader(form.Encode()))
	cross.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	cross.Header.Set("Origin", "http://evil.example")
	cross.AddCookie(c.cookie)
	if rec := do(s, cross); rec.Code != http.StatusForbidden {
		t.Errorf("cross-origin: %d, want 403", rec.Code)
	}
	if len(f.calls) != 0 {
		t.Fatalf("a refused request started %d runs", len(f.calls))
	}
}

func TestARunRecordsEveryStageAndItsOutput(t *testing.T) {
	s, f := newTestServer(t)
	f.fail = "lint"
	c := signIn(t, s)
	// The form's order and an unknown stage are ignored: pipeline order, known
	// stages only.
	id := c.startRun("lint", "nope", "detect")
	run := waitDone(t, s, id)

	if got := strings.Join(f.calls[0].Stages, ","); got != "detect,lint" {
		t.Errorf("stages run: %q, want detect,lint", got)
	}
	if run.Status != RunFailed {
		t.Errorf("status %q, want failed", run.Status)
	}
	if n := run.Counts(); n[result.StatusPass] != 2 || n[result.StatusFail] != 2 {
		t.Errorf("counts %v, want 2 pass and 2 fail", n)
	}
	if len(run.Lines) != 4 {
		t.Errorf("%d lines kept, want 4", len(run.Lines))
	}
	page := c.get("/runs/" + id).Body.String()
	for _, want := range []string{"lint failed", "detect ok", "output of lint", `data-live="0"`} {
		if !strings.Contains(page, want) {
			t.Errorf("run page lacks %q", want)
		}
	}
}

func TestOneRunAtATimeAndCancel(t *testing.T) {
	s, f := newTestServer(t)
	f.block = true
	c := signIn(t, s)
	id := c.startRun("detect")
	if rec := c.post("/runs", url.Values{"stage": {"detect"}}); rec.Code != http.StatusConflict ||
		!strings.Contains(rec.Body.String(), "/runs/"+id) {
		t.Errorf("second run: %d, want 409 linking the first", rec.Code)
	}
	if rec := c.post("/runs/"+id+"/cancel", nil); rec.Code != http.StatusSeeOther {
		t.Errorf("cancel: %d", rec.Code)
	}
	if run := waitDone(t, s, id); run.Status != RunCanceled {
		t.Errorf("status %q, want canceled", run.Status)
	}
}

func TestEventsStreamLinesStagesAndDone(t *testing.T) {
	s, _ := newTestServer(t)
	c := signIn(t, s)
	id := c.startRun("detect")
	waitDone(t, s, id)
	rec := c.get("/runs/" + id + "/events?after=1")
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}
	body := rec.Body.String()
	if strings.Count(body, "event: line") != 1 { // two lines, one after seq 1
		t.Errorf("want one line event after seq 1:\n%s", body)
	}
	if strings.Count(body, "event: stage") != 2 || !strings.Contains(body, "event: done") {
		t.Errorf("want two stage events and done:\n%s", body)
	}
}

func TestReportIsTheRunAsJSON(t *testing.T) {
	s, _ := newTestServer(t)
	c := signIn(t, s)
	id := c.startRun("detect")
	waitDone(t, s, id)
	rec := c.get("/runs/" + id + "/report.json")
	var got Run
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != id || got.Status != RunPassed || len(got.Repos) != 2 || got.Lines != nil {
		t.Errorf("report: %+v", got)
	}
}

// Finished runs are saved and a new server lists them; a run the server never
// finished comes back canceled.
func TestHistorySurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	s := newTestServerWith(t, Options{HistoryDir: dir})
	c := signIn(t, s)
	id := c.startRun("detect")
	waitDone(t, s, id)

	stale := Run{ID: "20260101-000000-abcdef", Status: RunRunning}
	body, _ := json.Marshal(stale)
	if err := os.WriteFile(filepath.Join(dir, stale.ID+".json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "garbage.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}

	again := newTestServerWith(t, Options{HistoryDir: dir})
	runs := again.runs.list()
	if len(runs) != 2 || runs[0].ID != id {
		t.Fatalf("reloaded %d runs: %+v", len(runs), runs)
	}
	if runs[1].Status != RunCanceled {
		t.Errorf("unfinished run reloaded as %q, want canceled", runs[1].Status)
	}
}

func TestReadOnlyHasNoRunRoutes(t *testing.T) {
	f := newFakeRunner()
	s := newTestServerWith(t, Options{Runner: f, ReadOnly: true})
	c := signIn(t, s)
	if rec := c.post("/runs", url.Values{"stage": {"detect"}}); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("start under --read-only: %d, want 405", rec.Code)
	}
	if strings.Contains(c.get("/").Body.String(), `action="/runs"`) {
		t.Error("the overview offers the run form under --read-only")
	}
	if len(f.calls) != 0 {
		t.Error("a run started under --read-only")
	}
}

func TestRunIDsCannotNameAPath(t *testing.T) {
	for _, id := range []string{"", "../../etc/passwd", "20260101-000000-ABCDEF", "20260101-000000-abcdef/x", "20260101x000000-abcdef"} {
		if validRunID(id) {
			t.Errorf("validRunID(%q) = true", id)
		}
	}
	if !validRunID("20260101-000000-abcdef") {
		t.Error("a well-formed ID is refused")
	}
	s, _ := newTestServer(t)
	c := signIn(t, s)
	if rec := c.get("/runs/..%2f..%2fetc"); rec.Code != http.StatusNotFound {
		t.Errorf("traversal: %d", rec.Code)
	}
}

func vendorFile(t *testing.T, name string) []byte {
	t.Helper()
	b, err := assetFS.ReadFile("assets/vendor/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Every vendored file hashes to what MANIFEST.sha256 records, so a hand edit or
// a partial update fails here. The set is icon-sets-sync's, copied byte for
// byte with its manifest.
func TestVendoredFilesMatchTheManifest(t *testing.T) {
	sc := bufio.NewScanner(bytes.NewReader(vendorFile(t, "MANIFEST.sha256")))
	n := 0
	for sc.Scan() {
		sum, name, ok := strings.Cut(sc.Text(), "  ")
		if !ok {
			t.Fatalf("bad manifest line %q", sc.Text())
		}
		got := sha256.Sum256(vendorFile(t, name))
		if hex.EncodeToString(got[:]) != sum {
			t.Errorf("%s does not match MANIFEST.sha256", name)
		}
		n++
	}
	if n < 10 {
		t.Fatalf("manifest lists %d files, want at least 10", n)
	}
}

// third-party-notices.md at the repository root carries every embedded
// license, so the release archives ship the notices the binary serves.
func TestNoticesCarryTheEmbeddedLicenses(t *testing.T) {
	s, _ := newTestServer(t)
	ls, err := s.embeddedLicenses()
	if err != nil {
		t.Fatal(err)
	}
	if len(ls) < 4 {
		t.Fatalf("%d licenses embedded", len(ls))
	}
	notices, err := os.ReadFile(filepath.Join("..", "..", "..", "third-party-notices.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range ls {
		if !strings.Contains(l.Text, "Permission is hereby granted") {
			t.Errorf("%s is not MIT", l.File)
		}
		if !bytes.Contains(notices, []byte(strings.TrimSpace(l.Text))) {
			t.Errorf("third-party-notices.md lacks the text of %s", l.File)
		}
	}
}

func TestDurationsReadAsTheyShould(t *testing.T) {
	dur := funcs(&assetSet{})["dur"].(func(time.Duration) string)
	for d, want := range map[time.Duration]string{
		0: "–", 300 * time.Microsecond: "<1ms", 42 * time.Millisecond: "42ms", 5612 * time.Millisecond: "5.6s",
	} {
		if got := dur(d); got != want {
			t.Errorf("dur(%v) = %q, want %q", d, got, want)
		}
	}
}
