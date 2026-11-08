package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestReadableHandlerWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := (&readableHandler{w: &buf, level: slog.LevelInfo}).WithAttrs([]slog.Attr{slog.String("run", "abc")})
	lg := slog.New(h)
	lg.Info("hello", "stage", "secrets")
	out := buf.String()
	for _, want := range []string{"hello", "run=abc", "stage=secrets", "INFO"} {
		if !strings.Contains(out, want) {
			t.Errorf("log line missing %q: %q", want, out)
		}
	}
}

func TestNewWritesReadableFile(t *testing.T) {
	dir := t.TempDir()
	lg, c := New(Options{Dir: dir, Command: "run", Repo: "x", RunID: "rid", Now: time.Now(), Level: slog.LevelInfo})
	lg.Info("started", "k", "v")
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRedactHandlerEnabled(t *testing.T) {
	h := &redactHandler{inner: &readableHandler{w: &bytes.Buffer{}, level: slog.LevelInfo}, redact: func(s string) string { return s }}
	if !h.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("info should be enabled")
	}
}
