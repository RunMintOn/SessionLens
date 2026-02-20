package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Source colors from the task requirements
var (
	openCodeColor = lipgloss.Color("#86EFAC") // light green
	claudeColor   = lipgloss.Color("#FDBA74") // orange
	qwenColor     = lipgloss.Color("#93C5FD") // light blue

	// Mac terminal style colors
	bgColor       = lipgloss.Color("#1E1E1E") // dark gray background
	borderColor   = lipgloss.Color("#3C3C3C") // gray border
	textColor     = lipgloss.Color("#FAFAFA") // white text
	selectedColor = lipgloss.Color("#569CD6") // blue highlight
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

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(borderColor).
			Background(bgColor).
			Padding(1, 2)

	headerPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder(), true, false, false, true).
				BorderForeground(borderColor).
				Background(bgColor).
				Padding(0, 1)

	footerPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder(), false, true, true, true).
				BorderForeground(borderColor).
				Background(bgColor).
				Padding(0, 1)

	searchBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(lipgloss.Color("#569CD6")).
			Background(bgColor).
			Padding(0, 1)

	searchPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#569CD6"))

	searchInputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF"))
)

type Session struct {
	id       string
	source   string
	messages int
}

type model struct {
	cursor    int
	sessions  []Session
	searching bool
	query     string
	width     int
	height    int
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		// Handle search mode input
		if m.searching {
			switch msg.String() {
			case "esc":
				m.searching = false
				m.query = ""
				// Reset cursor if out of bounds
				if m.cursor >= len(m.sessions) {
					m.cursor = 0
				}
			case "enter":
				m.searching = false
			case "backspace":
				if len(m.query) > 0 {
					m.query = m.query[:len(m.query)-1]
				}
				// Reset cursor when query changes
				m.cursor = 0
			default:
				// Add character to query
				m.query += msg.String()
				// Reset cursor when query changes
				m.cursor = 0
			}
			return m, nil
		}

		// Normal mode
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "/":
			m.searching = true
			m.query = ""
			m.cursor = 0
		case "enter":
			// Attach to the selected tmux session
			if m.cursor < len(m.sessions) {
				sessionID := m.sessions[m.cursor].id
				cmd := exec.Command("tmux", "attach-session", "-t", sessionID)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Run()
				return m, tea.Quit
			}
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
	filteredSessions := m.sessions
	if m.query != "" {
		queryLower := strings.ToLower(m.query)
		var filtered []Session
		for _, s := range m.sessions {
			if strings.Contains(strings.ToLower(s.id), queryLower) ||
				strings.Contains(strings.ToLower(s.source), queryLower) {
				filtered = append(filtered, s)
			}
		}
		filteredSessions = filtered
	}

	// Calculate dynamic dimensions
	headerHeight := 1
	if m.searching {
		headerHeight = 2
	}
	footerHeight := 1

	// Use provided dimensions or sensible defaults
	width := m.width
	height := m.height
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}

	// Calculate available content height
	contentHeight := height - headerHeight - footerHeight - 2 // -2 for padding

	header := headerPanelStyle.Width(width - 2).Render(titleStyle.Render(" Agent Session Manager "))

	if m.searching {
		searchBox := searchBoxStyle.Width(width - 4).Render(
			searchPromptStyle.Render("/ ") + searchInputStyle.Render(m.query),
		)
		header = header + "\n" + searchBox
	}

	var s string
	if m.query != "" {
		s += fmt.Sprintf("Sessions (%d/%d):\n\n", len(filteredSessions), len(m.sessions))
	} else {
		s += "Sessions:\n\n"
	}

	// Limit displayed sessions to fit in viewport
	displaySessions := filteredSessions
	if contentHeight > 0 && len(filteredSessions) > contentHeight-2 {
		displaySessions = filteredSessions[:contentHeight-2]
	}

	for i, session := range displaySessions {
		cursor := "  "
		style := itemStyle
		if m.cursor == i {
			cursor = "> "
			style = selectedStyle.Copy().Background(selectedColor)
		}

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

	sessionPanel := panelStyle.Width(width - 2).Height(contentHeight).Render(s)

	footerHelp := "↑/k up  ↓/j down  Enter attach  q quit"
	if m.searching {
		footerHelp = "type to filter  Esc exit"
	}
	footer := footerPanelStyle.Width(width - 2).Render(
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Render(footerHelp),
	)

	return header + "\n" + sessionPanel + "\n" + footer
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
		width:  0,
		height: 0,
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
