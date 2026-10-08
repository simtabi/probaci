package secret

import (
	"strings"
	"testing"
)

func TestRedactLiterals(t *testing.T) {
	r := New()
	r.Add("supersecretvalue", "abc") // "abc" too short -> ignored
	got := r.Redact("token=supersecretvalue rest")
	if got != "token="+Mask+" rest" {
		t.Fatalf("literal not redacted: %q", got)
	}
	if r.Redact("abc only") != "abc only" {
		t.Fatalf("short literal should not be masked")
	}
}

func TestRedactTokenShapes(t *testing.T) {
	// Token-shaped values are assembled at run time so the source carries no
	// literal a secret scanner matches by prefix.
	cases := []string{
		"ghp_" + strings.Repeat("x", 36),
		"glpat-" + strings.Repeat("x", 20),
		"AKIA" + strings.Repeat("X", 16),
		"Bearer abcdefghijklmnopqrstuvwxyz",
	}
	r := New()
	for _, c := range cases {
		if got := r.Redact("x " + c + " y"); got != "x "+Mask+" y" {
			t.Errorf("token %q not redacted: got %q", c, got)
		}
	}
}

func TestRedactEmpty(t *testing.T) {
	if New().Redact("") != "" {
		t.Fatal("empty should stay empty")
	}
}
