package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Source colors from the task requirements
var (
	openCodeColor = lipgloss.Color("#86EFAC") // light green
	claudeColor   = lipgloss.Color("#FDBA74") // orange
	qwenColor     = lipgloss.Color("#93C5FD") // light blue
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	itemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#3C3C3C"))

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FDBA74"))
)

type Session struct {
	id       string
	source   string
	messages int
}

type model struct {
	cursor   int
	sessions []Session
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.sessions)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	// Header
	header := titleStyle.Render(" Agent Session Manager ") + "\n\n"
	
	// Session list
	var s string
	s += "Sessions:\n\n"
	
	for i, session := range m.sessions {
		cursor := "  "
		style := itemStyle
		if m.cursor == i {
			cursor = "> "
			style = selectedStyle
		}
		
		// Color by source
		var sourceStyle lipgloss.Style
		switch session.source {
		case "OpenCode":
			sourceStyle = lipgloss.NewStyle().Foreground(openCodeColor)
		case "Claude":
			sourceStyle = lipgloss.NewStyle().Foreground(claudeColor)
		case "Qwen":
			sourceStyle = lipgloss.NewStyle().Foreground(qwenColor)
		default:
			sourceStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
		}
		
		line := fmt.Sprintf("%s%s  %s  (%d msgs)", 
			cursor, 
			session.id,
			sourceStyle.Render(session.source),
			session.messages,
		)
		s += style.Render(line) + "\n"
	}
	
	// Footer
	footer := "\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Render("↑/k up  ↓/j down  q quit")
	
	return header + s + footer
}

func main() {
	m := model{
		cursor: 0,
		sessions: []Session{
			{"ses_001", "OpenCode", 45},
			{"ses_002", "Claude", 23},
			{"ses_003", "Qwen", 12},
			{"ses_004", "OpenCode", 8},
			{"ses_005", "Claude", 31},
		},
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
