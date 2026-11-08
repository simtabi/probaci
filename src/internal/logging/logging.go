// Package logging configures probaci's structured logger. Each invocation
// writes its own log file (grouped into a per-day directory) so concurrent
// processes never contend on a shared file and any run is easy to attribute.
// The format is tuned for human readability, not raw JSON.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Options configures the logger.
type Options struct {
	// Dir is the logs base directory (e.g. ~/.config/probaci/logs).
	Dir string
	// Level is the minimum level to record.
	Level slog.Level
	// RunID, Command, and Repo make the per-run filename readable and unique:
	//   <Dir>/<YYYY-MM-DD>/<HH-MM-SS>_<command>_<repo>_<runid>.log
	RunID   string
	Command string
	Repo    string
	// Now is the invocation time (inject time.Now()); zero falls back to it.
	Now time.Time
	// RetentionDays prunes day-directories older than this on startup (0 = 14).
	RetentionDays int
	// Redactor, if set, scrubs secret-shaped strings from every record.
	Redactor func(string) string
}

// New builds a slog.Logger writing a readable, per-run log file. If Dir is empty
// it discards output, so the tool still runs without a home directory. The
// returned io.Closer closes the underlying file.
func New(opts Options) (*slog.Logger, io.Closer) {
	if opts.Dir == "" {
		return slog.New(slog.NewTextHandler(io.Discard, nil)), nopCloser{}
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	if opts.RetentionDays <= 0 {
		opts.RetentionDays = 14
	}
	prune(opts.Dir, opts.RetentionDays, now)

	dayDir := filepath.Join(opts.Dir, now.Format("2006-01-02"))
	if err := os.MkdirAll(dayDir, 0o755); err != nil {
		return slog.New(slog.NewTextHandler(io.Discard, nil)), nopCloser{}
	}
	name := fmt.Sprintf("%s_%s_%s_%s.log",
		now.Format("15-04-05"), slug(opts.Command, "run"), slug(opts.Repo, "repo"), slug(opts.RunID, "run"))
	f, err := os.OpenFile(filepath.Join(dayDir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return slog.New(slog.NewTextHandler(io.Discard, nil)), nopCloser{}
	}

	var handler slog.Handler = &readableHandler{w: f, level: opts.Level}
	if opts.Redactor != nil {
		handler = &redactHandler{inner: handler, redact: opts.Redactor}
	}
	return slog.New(handler), f
}

// slug makes a filename-safe token, falling back to def when empty.
func slug(s, def string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		s = def
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}

// prune removes day-directories older than retentionDays.
func prune(dir string, retentionDays int, now time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := now.AddDate(0, 0, -retentionDays)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		day, err := time.Parse("2006-01-02", e.Name())
		if err != nil {
			continue // not a day directory
		}
		if day.Before(cutoff) {
			_ = os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
}

// readableHandler writes aligned, human-readable lines:
//
//	2026-06-18 14:35:07.123  INFO   stage=secrets repo=api  message text
type readableHandler struct {
	w      io.Writer
	level  slog.Level
	groups string
	attrs  []slog.Attr // accumulated via WithAttrs
}

func (h *readableHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level
}

func (h *readableHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Time.Format("2006-01-02 15:04:05.000"))
	b.WriteString("  ")
	fmt.Fprintf(&b, "%-5s", r.Level.String())
	b.WriteString("  ")
	b.WriteString(r.Message)
	writeAttr := func(a slog.Attr) {
		b.WriteString("  ")
		if h.groups != "" {
			b.WriteString(h.groups)
		}
		b.WriteString(a.Key)
		b.WriteString("=")
		b.WriteString(a.Value.String())
	}
	for _, a := range h.attrs { // attrs attached via logger.With(...)
		writeAttr(a)
	}
	r.Attrs(func(a slog.Attr) bool { writeAttr(a); return true })
	b.WriteByte('\n')
	_, err := io.WriteString(h.w, b.String())
	return err
}

func (h *readableHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	merged = append(merged, h.attrs...)
	merged = append(merged, attrs...)
	return &readableHandler{w: h.w, level: h.level, groups: h.groups, attrs: merged}
}

func (h *readableHandler) WithGroup(name string) slog.Handler {
	return &readableHandler{w: h.w, level: h.level, groups: h.groups + name + ".", attrs: h.attrs}
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

// redactHandler wraps another handler, scrubbing the message and string-valued
// attributes through a redactor before delegating.
type redactHandler struct {
	inner  slog.Handler
	redact func(string) string
}

func (h *redactHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.inner.Enabled(ctx, l)
}

func (h *redactHandler) Handle(ctx context.Context, r slog.Record) error {
	clean := slog.NewRecord(r.Time, r.Level, h.redact(r.Message), r.PC)
	r.Attrs(func(a slog.Attr) bool {
		clean.AddAttrs(h.redactAttr(a))
		return true
	})
	return h.inner.Handle(ctx, clean)
}

func (h *redactHandler) redactAttr(a slog.Attr) slog.Attr {
	if a.Value.Kind() == slog.KindString {
		return slog.String(a.Key, h.redact(a.Value.String()))
	}
	return a
}

func (h *redactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &redactHandler{inner: h.inner.WithAttrs(attrs), redact: h.redact}
}

func (h *redactHandler) WithGroup(name string) slog.Handler {
	return &redactHandler{inner: h.inner.WithGroup(name), redact: h.redact}
}
