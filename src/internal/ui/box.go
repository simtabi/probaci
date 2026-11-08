package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Box renders content inside a rounded frame (ASCII fallback when Unicode is
// unavailable), with an optional title line at the top.
func (t *Theme) Box(title, body string) string {
	border := lipgloss.RoundedBorder()
	if !t.Unicode {
		border = lipgloss.Border{
			Top: "-", Bottom: "-", Left: "|", Right: "|",
			TopLeft: "+", TopRight: "+", BottomLeft: "+", BottomRight: "+",
		}
	}
	style := lipgloss.NewStyle().Border(border).Padding(0, 1)
	if t.Color {
		style = style.BorderForeground(lipgloss.Color("4"))
	}
	content := body
	if title != "" {
		content = t.Bold(title) + "\n" + body
	}
	return style.Render(content)
}

// KeyValue renders aligned key/value lines for status panels.
func (t *Theme) KeyValue(pairs [][2]string) string {
	width := 0
	for _, p := range pairs {
		if len(p[0]) > width {
			width = len(p[0])
		}
	}
	var b strings.Builder
	for i, p := range pairs {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(t.dim.Render(pad(p[0], width)))
		b.WriteString("  ")
		b.WriteString(p[1])
	}
	return b.String()
}

func pad(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}
