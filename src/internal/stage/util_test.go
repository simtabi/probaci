package stage

import (
	"testing"

	"github.com/simtabi/probaci/internal/result"
)

func TestWorsen(t *testing.T) {
	cases := []struct {
		a, b, want result.Status
	}{
		{result.StatusSkip, result.StatusPass, result.StatusPass},
		{result.StatusPass, result.StatusFail, result.StatusFail},
		{result.StatusFail, result.StatusError, result.StatusError},
		{result.StatusError, result.StatusPass, result.StatusError},
	}
	for _, c := range cases {
		if got := worsen(c.a, c.b); got != c.want {
			t.Errorf("worsen(%s,%s)=%s want %s", c.a, c.b, got, c.want)
		}
	}
}

func TestGrepVersion(t *testing.T) {
	yaml := "jobs:\n  build:\n    steps:\n      - uses: actions/setup-node\n        with:\n          node-version: 20.11\n"
	if v := grepVersion(yaml, "node-version"); v != "20.11" {
		t.Fatalf("grepVersion=%q want 20.11", v)
	}
	if v := grepVersion(yaml, "python-version"); v != "" {
		t.Fatalf("expected empty for missing key, got %q", v)
	}
}
