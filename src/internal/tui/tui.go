// Package tui is the Bubble Tea dashboard. It runs the same stage engine the
// CLI uses, subscribing to its event stream to show live per-repo/stage status
// and streaming logs — no logic is duplicated from the runner.
package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/simtabi/probaci/internal/result"
	"github.com/simtabi/probaci/internal/stage"
	"github.com/simtabi/probaci/internal/ui"
)

// Run launches the dashboard for the given engine and options.
func Run(engine *stage.Engine, opts stage.RunOptions) error {
	m := newModel(engine, opts)
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

type row struct {
	repo   string
	stage  string
	status result.Status
}

type eventMsg stage.Event
type finishedMsg struct{ agg result.Aggregate }

type model struct {
	engine *stage.Engine
	opts   stage.RunOptions

	ctx    context.Context
	cancel context.CancelFunc
	events chan stage.Event
	done   chan result.Aggregate

	rows  []row
	index map[string]int
	logs  []string

	spin     spinner.Model
	theme    *ui.Theme
	finished bool
	agg      result.Aggregate
	w, h     int
}

func newModel(engine *stage.Engine, opts stage.RunOptions) model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	ctx, cancel := context.WithCancel(context.Background())
	return model{
		engine: engine,
		opts:   opts,
		ctx:    ctx,
		cancel: cancel,
		events: make(chan stage.Event, 256),
		done:   make(chan result.Aggregate, 1),
		index:  map[string]int{},
		spin:   sp,
		theme:  ui.Detect(true, false),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, m.start(), m.listen())
}

// start kicks off the engine in a goroutine, funneling events to the channel.
func (m model) start() tea.Cmd {
	return func() tea.Msg {
		go func() {
			agg := m.engine.Run(m.ctx, m.opts, func(ev stage.Event) {
				m.events <- ev
			})
			m.done <- agg
			close(m.events)
		}()
		return nil
	}
}

// listen pulls one event (or the finished signal) and turns it into a message.
func (m model) listen() tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-m.events
		if !ok {
			return finishedMsg{agg: <-m.done}
		}
		return eventMsg(ev)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.cancel() // stop the engine (kills any running container) before quitting
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case eventMsg:
		m.applyEvent(stage.Event(msg))
		return m, m.listen()
	case finishedMsg:
		m.finished = true
		m.agg = msg.agg
		return m, nil
	}
	return m, nil
}

func (m *model) applyEvent(ev stage.Event) {
	key := ev.Repo + "|" + ev.Stage
	if _, ok := m.index[key]; !ok {
		m.index[key] = len(m.rows)
		m.rows = append(m.rows, row{repo: ev.Repo, stage: ev.Stage, status: result.StatusPending})
	}
	i := m.index[key]
	if ev.Result != nil {
		m.rows[i].status = ev.Result.Status
	} else if ev.Status != "" {
		m.rows[i].status = ev.Status
	}
	if ev.Line != "" {
		m.logs = append(m.logs, strings.Split(ev.Line, "\n")...)
		if len(m.logs) > 500 {
			m.logs = m.logs[len(m.logs)-500:]
		}
	}
}

func (m model) View() string {
	left := m.renderStages()
	right := m.renderLogs()
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
	return body + "\n" + m.footer()
}

func (m model) renderStages() string {
	var b strings.Builder
	b.WriteString(m.theme.Heading("stages") + "\n")
	lastRepo := ""
	for _, r := range m.rows {
		if r.repo != lastRepo {
			b.WriteString(m.theme.Bold(filepath.Base(r.repo)) + "\n")
			lastRepo = r.repo
		}
		glyph := m.theme.Glyph(r.status)
		if r.status == result.StatusRunning {
			glyph = m.spin.View()
		}
		fmt.Fprintf(&b, "  %s %s\n", glyph, r.stage)
	}
	return lipgloss.NewStyle().Width(34).Render(b.String())
}

func (m model) renderLogs() string {
	height := m.h - 4
	if height < 5 {
		height = 5
	}
	start := 0
	if len(m.logs) > height {
		start = len(m.logs) - height
	}
	body := strings.Join(m.logs[start:], "\n")
	return lipgloss.NewStyle().Width(maxInt(20, m.w-40)).Render(m.theme.Heading("logs") + "\n" + body)
}

func (m model) footer() string {
	if m.finished {
		status := m.theme.Pass("all clear — safe to push")
		if m.agg.Failed() {
			status = m.theme.Fail("stages failed — fix before pushing")
		}
		return m.theme.Dim("done · ") + status + m.theme.Dim("  ·  q quit")
	}
	return m.theme.Dim("running…  ·  q quit")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
