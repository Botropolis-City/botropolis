package tui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"botropolis.city/botropolis/pkg/state"
)

type Controller interface {
	Attach(id string) error
	Stop(ctx context.Context, id string) error
	Resume(ctx context.Context, dir, sessionID string) (string, error)
	Remove(ctx context.Context, id string) error
}

type snapshotMsg state.Snapshot

type program struct {
	model      *Model
	controller Controller
	updates    <-chan state.Snapshot
	width      int
	height     int
	clock      func() time.Time
}

var (
	styleHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("250"))
	styleCursor = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("237"))
	styleNeeds  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleFooter = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

// Run shows the sessions until the person quits, or until the feed
// refuses, such as on meeting a daemon out of sync (item 152), whose
// refusal it returns.
func Run(ctx context.Context, feed func(ctx context.Context, offer func(state.Snapshot)) error, controller Controller) error {
	updates := make(chan state.Snapshot, 1)
	runCtx, stop := context.WithCancelCause(ctx)
	defer stop(nil)
	feedCtx, cancel := context.WithCancel(runCtx)
	defer cancel()
	go func() {
		if err := feed(feedCtx, offerLatest(updates)); err != nil {
			stop(err)
		}
	}()
	p := &program{model: New(), controller: controller, updates: updates, clock: time.Now}
	_, err := tea.NewProgram(p, tea.WithContext(runCtx), tea.WithAltScreen()).Run()
	if cause := context.Cause(runCtx); cause != nil && ctx.Err() == nil {
		return cause
	}
	return err
}

// offerLatest keeps only the newest snapshot waiting.
func offerLatest(updates chan state.Snapshot) func(state.Snapshot) {
	return func(s state.Snapshot) {
		select {
		case updates <- s:
		default:
			select {
			case <-updates:
			default:
			}
			updates <- s
		}
	}
}

func (p *program) Init() tea.Cmd {
	return p.wait()
}

func (p *program) wait() tea.Cmd {
	return func() tea.Msg {
		s, ok := <-p.updates
		if !ok {
			return nil
		}
		return snapshotMsg(s)
	}
}

func (p *program) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case snapshotMsg:
		p.model.SetSnapshot(state.Snapshot(msg))
		return p, p.wait()
	case tea.WindowSizeMsg:
		p.width, p.height = msg.Width, msg.Height
		p.model.SetHeight(msg.Height)
	case tea.KeyMsg:
		action := p.model.Key(msg.String(), p.clock())
		switch action.Kind {
		case ActionQuit:
			return p, tea.Quit
		case ActionNone:
		default:
			p.perform(action)
		}
	}
	return p, nil
}

func (p *program) perform(action Action) {
	ctx := context.Background()
	var err error
	switch action.Kind {
	case ActionAttach:
		err = p.controller.Attach(action.SessionID)
		p.model.SetStatus("attached " + action.SessionID[:8])
	case ActionStop:
		err = p.controller.Stop(ctx, action.SessionID)
		p.model.SetStatus("stopped " + action.SessionID[:8])
	case ActionResume:
		_, err = p.controller.Resume(ctx, "", action.SessionID)
		p.model.SetStatus("resumed " + action.SessionID[:8])
	case ActionDemolish:
		err = p.controller.Remove(ctx, action.SessionID)
	}
	if err != nil {
		p.model.SetStatus(err.Error())
	}
}

func (p *program) View() string {
	lines := p.model.Lines(p.width)
	var b strings.Builder
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "  STATE "):
			b.WriteString(styleHeader.Render(line))
		case strings.HasPrefix(line, "> "):
			b.WriteString(styleCursor.Render(line))
		case strings.HasPrefix(line, "  ↑ "):
			b.WriteString(styleFooter.Render(line))
		case strings.HasPrefix(strings.TrimSpace(line), string(state.NeedsYou)):
			b.WriteString(styleNeeds.Render(line))
		default:
			b.WriteString(line)
		}
		b.WriteString("\n")
	}
	if p.height > 0 {
		for i := len(lines); i < p.height-1; i++ {
			b.WriteString("\n")
		}
	}
	b.WriteString(styleFooter.Render(p.model.Footer()))
	return b.String()
}
