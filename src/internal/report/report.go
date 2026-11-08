// Package report turns an aggregate run result into human (text) or machine
// (JSON) output. All strings here have already been redacted by the engine.
package report

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/simtabi/probaci/internal/result"
	"github.com/simtabi/probaci/internal/ui"
)

// JSON renders the aggregate as indented JSON for machine consumption.
func JSON(agg result.Aggregate) ([]byte, error) {
	return json.MarshalIndent(agg, "", "  ")
}

// Text renders a human-readable per-repo + aggregate summary.
func Text(agg result.Aggregate, t *ui.Theme) string {
	var b strings.Builder
	totalFail := 0
	for _, repo := range agg.Repos {
		fmt.Fprintf(&b, "\n%s %s\n", t.Heading("repo"), t.Bold(repo.Path))
		for _, r := range repo.Results {
			line := fmt.Sprintf("  %s %-15s %s",
				t.Glyph(r.Status), r.Stage, t.Dim(dur(r.Duration)))
			if r.Summary != "" {
				line += "  " + r.Summary
			}
			fmt.Fprintln(&b, line)
			if !r.Status.OK() {
				totalFail++
			}
		}
	}

	// Aggregate summary box.
	var rows [][2]string
	for _, repo := range agg.Repos {
		status := t.Pass("clear")
		if repo.Failed() {
			status = t.Fail("failed")
		}
		rows = append(rows, [2]string{repo.Path, status})
	}
	title := "ALL CLEAR — safe to push."
	if agg.Failed() {
		title = fmt.Sprintf("%d stage(s) failed — fix before pushing.", totalFail)
	}
	b.WriteString("\n")
	b.WriteString(t.Box(title, t.KeyValue(rows)))
	b.WriteString("\n")
	return b.String()
}

func dur(d time.Duration) string {
	if d == 0 {
		return ""
	}
	return d.Round(time.Millisecond).String()
}
