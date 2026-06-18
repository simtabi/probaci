package version

import (
	"strings"
	"testing"
)

func TestStringAndShort(t *testing.T) {
	if !strings.HasPrefix(String(), "probaci ") {
		t.Fatalf("String() should start with 'probaci ': %q", String())
	}
	if Short() == "" {
		t.Fatal("Short() must not be empty")
	}
}
