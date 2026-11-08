package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/simtabi/probaci/internal/result"
	"github.com/simtabi/probaci/internal/ui"
)

func sampleAgg() result.Aggregate {
	return result.Aggregate{Repos: []result.RepoReport{
		{Path: "/repo/a", Results: []result.Result{
			{Stage: "detect", Status: result.StatusPass, Summary: "ok", Duration: time.Millisecond},
			{Stage: "secrets", Status: result.StatusFail, Summary: "leak found"},
		}},
	}}
}

func TestJSONValid(t *testing.T) {
	data, err := JSON(sampleAgg())
	if err != nil {
		t.Fatal(err)
	}
	var back result.Aggregate
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("JSON not round-trippable: %v", err)
	}
	if !back.Failed() {
		t.Fatal("aggregate with a fail should report Failed()")
	}
}

func TestTextPlainRendersStagesAndSummary(t *testing.T) {
	th := ui.Detect(false, true) // plain, ASCII
	out := Text(sampleAgg(), th)
	for _, want := range []string{"/repo/a", "detect", "secrets", "failed"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("plain theme must not emit ANSI escapes")
	}
}

func TestTextAllClear(t *testing.T) {
	agg := result.Aggregate{Repos: []result.RepoReport{
		{Path: "/r", Results: []result.Result{{Stage: "detect", Status: result.StatusPass}}},
	}}
	out := Text(agg, ui.Detect(false, true))
	if !strings.Contains(out, "ALL CLEAR") {
		t.Errorf("expected ALL CLEAR for a passing run:\n%s", out)
	}
}
