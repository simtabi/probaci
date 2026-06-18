package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// bannerWidth is the inner content width of the masthead.
const bannerWidth = 56

// Banner renders the probaci masthead: a framed, centered wordmark with the
// tagline, name origin, and version. Color and rounded borders are used when
// the terminal supports them; otherwise it degrades to a plain ASCII box.
func (t *Theme) Banner(version string) string {
	center := lipgloss.NewStyle().Width(bannerWidth).Align(lipgloss.Center)

	lines := []string{
		"",
		center.Render(t.brand.Render(spaced("PROBACI"))),
		center.Render(t.dim.Render("Prove your pipeline before you push")),
		center.Render(t.dim.Render(`Latin probare - "to test / prove" - proh-BAH-see`)),
		center.Render(t.bold.Render(version) + t.dim.Render("  -  Simtabi")),
		"",
	}
	body := strings.Join(lines, "\n")

	border := lipgloss.RoundedBorder()
	if !t.Unicode {
		border = lipgloss.Border{
			Top: "-", Bottom: "-", Left: "|", Right: "|",
			TopLeft: "+", TopRight: "+", BottomLeft: "+", BottomRight: "+",
		}
	}
	frame := lipgloss.NewStyle().Border(border).Padding(0, 1)
	if t.Color {
		frame = frame.BorderForeground(lipgloss.Color("13"))
	}
	return frame.Render(body)
}

// spaced inserts a single space between runes for a letter-spaced wordmark.
func spaced(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}
