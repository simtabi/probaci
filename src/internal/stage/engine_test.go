package stage

import (
	"context"
	"testing"

	"github.com/simtabi/probaci/internal/config"
	"github.com/simtabi/probaci/internal/result"
	"github.com/simtabi/probaci/internal/tool"
)

// TestPerRepoConfigSelectsStages verifies that a per-repo resolver controls
// which stages run for that repo (only `detect` enabled here), independent of
// the base config.
func TestPerRepoConfigSelectsStages(t *testing.T) {
	base := config.Default()
	e := New(base, nil, tool.New(nil), nil)

	repoCfg := config.Default()
	for i := range repoCfg.Stages {
		repoCfg.Stages[i].Enabled = repoCfg.Stages[i].Name == "detect"
	}
	e.SetRepoConfig(func(string) (config.Config, *tool.Registry) {
		return repoCfg, tool.New(nil)
	})

	var stages []string
	agg := e.Run(context.Background(), RunOptions{Repos: []string{t.TempDir()}}, func(ev Event) {
		if ev.Result != nil {
			stages = append(stages, ev.Stage)
		}
	})
	if len(agg.Repos) != 1 {
		t.Fatalf("expected 1 repo report, got %d", len(agg.Repos))
	}
	if len(stages) != 1 || stages[0] != "detect" {
		t.Fatalf("per-repo config should run only detect, ran %v", stages)
	}
}

func TestStageListOnlySkip(t *testing.T) {
	e := New(config.Default(), nil, tool.New(nil), nil)
	got := e.StageList([]string{"secrets", "lint", "audit"}, []string{"lint"})
	want := []string{"secrets", "audit"}
	if len(got) != len(want) || got[0] != "secrets" || got[1] != "audit" {
		t.Fatalf("StageList=%v want %v", got, want)
	}
}

func TestCanceledContextStopsRun(t *testing.T) {
	e := New(config.Default(), nil, tool.New(nil), nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-canceled
	agg := e.Run(ctx, RunOptions{Repos: []string{t.TempDir()}, Only: []string{"detect"}}, nil)
	for _, r := range agg.Repos[0].Results {
		if r.Status == result.StatusRunning {
			t.Fatal("no stage should run under a canceled context")
		}
	}
}
