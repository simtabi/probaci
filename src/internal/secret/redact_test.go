package secret

import (
	"crypto/rand"
	"testing"
)

// gen returns n characters drawn from alphabet with crypto/rand, so no test
// value is written down and a secret scanner has nothing to match.
func gen(t *testing.T, n int, alphabet string) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

const (
	lowerDigits = "abcdefghijklmnopqrstuvwxyz0123456789"
	upperDigits = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
)

func TestRedactLiterals(t *testing.T) {
	r := New()
	secret := gen(t, 16, lowerDigits)
	r.Add(secret, "abc") // "abc" too short -> ignored
	got := r.Redact("token=" + secret + " rest")
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
		"ghp_" + gen(t, 36, lowerDigits),
		"glpat-" + gen(t, 20, lowerDigits),
		"AKIA" + gen(t, 16, upperDigits),
		"Bearer " + gen(t, 26, lowerDigits),
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
