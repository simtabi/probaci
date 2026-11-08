package ui

import (
	"strings"
	"testing"
)

func TestBannerASCIIFallback(t *testing.T) {
	th := Detect(false, true) // forcePlain -> no color, ASCII box
	out := th.Banner("v1.2.3")
	if strings.ContainsAny(out, "╭╮╰╯─│") {
		t.Errorf("ASCII fallback should not contain Unicode box glyphs:\n%s", out)
	}
	for _, want := range []string{"+", "P R O B A C I", "Prove your pipeline", "v1.2.3", "Simtabi"} {
		if !strings.Contains(out, want) {
			t.Errorf("banner missing %q:\n%s", want, out)
		}
	}
}

func TestSpaced(t *testing.T) {
	if got := spaced("ABC"); got != "A B C" {
		t.Fatalf("spaced=%q", got)
	}
}
