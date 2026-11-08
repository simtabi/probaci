// Package ui centralizes terminal styling: a single Charm/lipgloss toolkit with
// status glyphs that fall back to ASCII when the locale or terminal can't
// render Unicode, and color that honors NO_COLOR and TTY detection (clig.dev).
package ui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/simtabi/probaci/internal/result"
)

// Theme holds resolved capability flags and styles.
type Theme struct {
	Color   bool
	Unicode bool

	pass    lipgloss.Style
	fail    lipgloss.Style
	warn    lipgloss.Style
	info    lipgloss.Style
	dim     lipgloss.Style
	bold    lipgloss.Style
	heading lipgloss.Style
	brand   lipgloss.Style
}

// Detect builds a Theme from the environment. forceColor/forcePlain override
// auto-detection (e.g. --ci forces plain).
func Detect(stdoutIsTTY bool, forcePlain bool) *Theme {
	color := stdoutIsTTY && os.Getenv("NO_COLOR") == "" && !forcePlain
	unicode := supportsUnicode() && !forcePlain
	t := &Theme{Color: color, Unicode: unicode}
	mk := func(c string) lipgloss.Style {
		s := lipgloss.NewStyle()
		if color {
			s = s.Foreground(lipgloss.Color(c))
		}
		return s
	}
	t.pass = mk("2") // green
	t.fail = mk("1") // red
	t.warn = mk("3") // yellow
	t.info = mk("6") // cyan
	t.dim = mk("8")  // gray
	t.bold = lipgloss.NewStyle().Bold(color)
	t.heading = lipgloss.NewStyle().Bold(color)
	t.brand = lipgloss.NewStyle().Bold(color)
	if color {
		t.heading = t.heading.Foreground(lipgloss.Color("4"))
		t.brand = t.brand.Foreground(lipgloss.Color("13")) // bright magenta
	}
	return t
}

func supportsUnicode() bool {
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := os.Getenv(key); strings.Contains(strings.ToUpper(v), "UTF") {
			return true
		}
	}
	// Windows Terminal and modern macOS terminals default to UTF-8.
	return os.Getenv("WT_SESSION") != "" || os.Getenv("TERM_PROGRAM") != ""
}

// Glyph returns the status indicator with an ASCII fallback.
func (t *Theme) Glyph(s result.Status) string {
	if t.Unicode {
		switch s {
		case result.StatusPass:
			return t.pass.Render("✓")
		case result.StatusFail:
			return t.fail.Render("✗")
		case result.StatusError:
			return t.fail.Render("✗")
		case result.StatusSkip:
			return t.dim.Render("•")
		case result.StatusRunning:
			return t.info.Render("●")
		default:
			return t.dim.Render("⏳")
		}
	}
	switch s {
	case result.StatusPass:
		return t.pass.Render("[OK]")
	case result.StatusFail, result.StatusError:
		return t.fail.Render("[FAIL]")
	case result.StatusSkip:
		return t.dim.Render("[SKIP]")
	case result.StatusRunning:
		return t.info.Render("[..]")
	default:
		return t.dim.Render("[?]")
	}
}

// Label returns an uppercase, colored status word.
func (t *Theme) Label(s result.Status) string {
	switch s {
	case result.StatusPass:
		return t.pass.Render("PASS")
	case result.StatusFail:
		return t.fail.Render("FAIL")
	case result.StatusError:
		return t.fail.Render("ERROR")
	case result.StatusSkip:
		return t.dim.Render("SKIP")
	default:
		return t.dim.Render(strings.ToUpper(string(s)))
	}
}

// Heading styles a section heading.
func (t *Theme) Heading(s string) string { return t.heading.Render(s) }

// Dim styles secondary text.
func (t *Theme) Dim(s string) string { return t.dim.Render(s) }

// Bold styles emphasized text.
func (t *Theme) Bold(s string) string { return t.bold.Render(s) }

// Warn styles a warning string.
func (t *Theme) Warn(s string) string { return t.warn.Render(s) }

// Pass styles a success string.
func (t *Theme) Pass(s string) string { return t.pass.Render(s) }

// Fail styles a failure string.
func (t *Theme) Fail(s string) string { return t.fail.Render(s) }

// Info styles an informational string.
func (t *Theme) Info(s string) string { return t.info.Render(s) }
