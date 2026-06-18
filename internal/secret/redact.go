// Package secret provides a central redactor that scrubs known secret values
// and token-shaped strings from any text before it reaches stdout, --json
// output, or the log file.
package secret

import (
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Mask is the replacement written in place of a redacted value.
const Mask = "«redacted»"

// tokenPatterns match common token shapes (GitHub, GitLab, generic bearer-ish
// long base64/hex blobs). These are deliberately conservative to avoid
// scrubbing ordinary output.
var tokenPatterns = []*regexp.Regexp{
	regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{16,}`),                                    // GitHub tokens
	regexp.MustCompile(`glpat-[A-Za-z0-9_-]{20,}`),                                      // GitLab PAT
	regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`),                                  // Slack
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),                                              // AWS access key id
	regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._-]{20,}`),                              // bearer headers
	regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`), // JWT
}

// Redactor scrubs registered literal values and token-shaped substrings.
type Redactor struct {
	mu      sync.RWMutex
	literal []string
}

// New returns an empty Redactor.
func New() *Redactor { return &Redactor{} }

// Add registers literal secret values (e.g. token contents read from a secrets
// file) to be masked verbatim. Empty and very short values are ignored to
// avoid masking incidental text.
func (r *Redactor) Add(values ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range values {
		if len(strings.TrimSpace(v)) >= 4 {
			r.literal = append(r.literal, v)
		}
	}
	// Longest first so overlapping literals mask completely.
	sort.Slice(r.literal, func(i, j int) bool { return len(r.literal[i]) > len(r.literal[j]) })
}

// Redact returns s with every registered literal and token-shaped match
// replaced by Mask.
func (r *Redactor) Redact(s string) string {
	if s == "" {
		return s
	}
	r.mu.RLock()
	literals := r.literal
	r.mu.RUnlock()
	for _, lit := range literals {
		s = strings.ReplaceAll(s, lit, Mask)
	}
	for _, re := range tokenPatterns {
		s = re.ReplaceAllString(s, Mask)
	}
	return s
}

// Func returns the Redact method as a standalone function value, convenient for
// wiring into the logger.
func (r *Redactor) Func() func(string) string { return r.Redact }
