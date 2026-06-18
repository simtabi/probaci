// Package run carries per-invocation identity. A RunID uniquely (and readably)
// distinguishes concurrent probaci processes so their logs, containers, and
// temp directories never collide.
package run

import (
	"os"
	"strconv"
	"time"
)

// ID identifies a single probaci invocation.
type ID struct {
	// Short is a compact, sortable token: base36(unix-nanos) + "-" + pid.
	Short string
	// Started is the invocation start time.
	Started time.Time
	// PID is the process id.
	PID int
}

// New mints a RunID for the current process. now is injected so callers and
// tests stay deterministic; pass time.Now().
func New(now time.Time) ID {
	pid := os.Getpid()
	token := strconv.FormatInt(now.UnixNano(), 36) + "-" + strconv.Itoa(pid)
	return ID{Short: token, Started: now, PID: pid}
}

// String returns the short token.
func (i ID) String() string { return i.Short }
