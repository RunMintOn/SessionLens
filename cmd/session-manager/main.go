package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"agent-session-manager/internal/hidden"
	"agent-session-manager/session"
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

	filterActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(selectedColor).
				Padding(0, 1)

	filterInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				Padding(0, 1)
)

type model struct {
	cursor            int
	sessions          []session.Session
	searching         bool
	query             string
	width             int
	height            int
	hiddenManager     *hidden.Manager
	showHiddenOverlay bool
	hiddenCursor      int
	sourceFilter      string // "", "opencode", "claude", "qwen"
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
		// Handle hidden overlay mode
		if m.showHiddenOverlay {
			hiddenIDs := []string{}
			if m.hiddenManager != nil {
				hiddenIDs = m.hiddenManager.List()
			}
			switch msg.String() {
			case "esc":
				m.showHiddenOverlay = false
				m.hiddenCursor = 0
			case "k", "up":
				if m.hiddenCursor > 0 {
					m.hiddenCursor--
				}
			case "j", "down":
				if len(hiddenIDs) > 0 && m.hiddenCursor < len(hiddenIDs)-1 {
					m.hiddenCursor++
				}
			case "r":
				// Restore selected hidden session
				if m.hiddenCursor < len(hiddenIDs) {
					if err := m.hiddenManager.Remove(hiddenIDs[m.hiddenCursor]); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to restore session: %v\n", err)
					}
					// Clamp cursor after removal
					if m.hiddenCursor >= len(hiddenIDs)-1 && len(hiddenIDs) > 1 {
						m.hiddenCursor = len(hiddenIDs) - 2
					}
				}
			case "a":
				// Restore all hidden sessions
				if m.hiddenManager != nil {
					for _, id := range hiddenIDs {
						if err := m.hiddenManager.Remove(id); err != nil {
							fmt.Fprintf(os.Stderr, "Failed to restore session %s: %v\n", id, err)
						}
					}
				}
				m.showHiddenOverlay = false
				m.hiddenCursor = 0
			}
			return m, nil
		}

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
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyUp:
			visibleSessions := m.getVisibleSessions()
			if m.cursor > 0 {
				m.cursor--
			}
			if m.cursor >= len(visibleSessions) && len(visibleSessions) > 0 {
				m.cursor = len(visibleSessions) - 1
			}
		case tea.KeyDown:
			visibleSessions := m.getVisibleSessions()
			if m.cursor < len(visibleSessions)-1 {
				m.cursor++
			}
		}

		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "/":
			m.searching = true
			m.query = ""
			m.cursor = 0
		case "enter":
			visibleSessions := m.getVisibleSessions()
			if m.cursor < len(visibleSessions) {
				sess := visibleSessions[m.cursor]
				sessionID := sess.ID
				tool := sess.SourceTool

				var restoreCmd *exec.Cmd
				switch tool {
				case session.SourceClaude:
					restoreCmd = exec.Command("claude", "-r", sessionID)
				case session.SourceOpenCode:
					restoreCmd = exec.Command("opencode", "-s", sessionID)
				case session.SourceQwen:
					restoreCmd = exec.Command("qwen", "-r", sessionID)
				default:
					return m, nil
				}

				if err := restoreCmd.Start(); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to launch %s: %v\n", tool, err)
				}
				return m, nil
			}
		case "k":
			visibleSessions := m.getVisibleSessions()
			if m.cursor > 0 {
				m.cursor--
			}
			if m.cursor >= len(visibleSessions) && len(visibleSessions) > 0 {
				m.cursor = len(visibleSessions) - 1
			}
		case "j":
			visibleSessions := m.getVisibleSessions()
			if m.cursor < len(visibleSessions)-1 {
				m.cursor++
			}
		case "h":
			visibleSessions := m.getVisibleSessions()
			if m.cursor < len(visibleSessions) && m.hiddenManager != nil {
				sess := visibleSessions[m.cursor]
				if err := m.hiddenManager.Add(sess.ID); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to hide session: %v\n", err)
				}
			}
		case "H":
			// Toggle hidden overlay
			m.showHiddenOverlay = true
			m.hiddenCursor = 0
		case "1":
			// Filter: All
			if m.sourceFilter != "" {
				m.sourceFilter = ""
				m.cursor = 0
			}
		case "2":
			// Filter: Claude
			if m.sourceFilter != "claude" {
				m.sourceFilter = "claude"
				m.cursor = 0
			}
		case "3":
			// Filter: OpenCode
			if m.sourceFilter != "opencode" {
				m.sourceFilter = "opencode"
				m.cursor = 0
			}
		case "4":
			// Filter: Qwen
			if m.sourceFilter != "qwen" {
				m.sourceFilter = "qwen"
				m.cursor = 0
			}
		}
	}
	return m, nil
}

type groupedSession struct {
	projectPath string
	sessions    []session.Session
}

// groupSessionsByProject groups sessions by ProjectPath and sorts groups
// by most recent session in each group.
func groupSessionsByProject(sessions []session.Session) []groupedSession {
	groups := make(map[string][]session.Session)
	for _, sess := range sessions {
		groups[sess.ProjectPath] = append(groups[sess.ProjectPath], sess)
	}

	var result []groupedSession
	for path, sessList := range groups {
		result = append(result, groupedSession{
			projectPath: path,
			sessions:    sessList,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		iLatest := int64(0)
		for _, s := range result[i].sessions {
			if s.LastUpdated > iLatest {
				iLatest = s.LastUpdated
			}
		}
		jLatest := int64(0)
		for _, s := range result[j].sessions {
			if s.LastUpdated > jLatest {
				jLatest = s.LastUpdated
			}
		}
		return iLatest > jLatest
	})

	return result
}

// simplifyPath returns the full project path with home directory replaced by ~.
// For example: "/home/user/project" -> "~/project"
func simplifyPath(fullPath string) string {
	if fullPath == "" {
		return "(no project)"
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fullPath
	}
	if strings.HasPrefix(fullPath, homeDir) {
		relPath := strings.TrimPrefix(fullPath, homeDir)
		relPath = strings.TrimLeft(relPath, "/")
		if relPath == "" {
			return "~"
		}
		return "~/" + relPath
	}
	return fullPath
}

// getVisibleSessions returns the list of sessions visible to the user,
// applying search query filtering, source filter, and hidden session filtering.
// This ensures Update() and View() use the same visible session list.
func (m model) getVisibleSessions() []session.Session {
	filteredSessions := m.sessions
	if m.query != "" {
		queryLower := strings.ToLower(m.query)
		var filtered []session.Session
		for _, s := range m.sessions {
			if strings.Contains(strings.ToLower(s.ID), queryLower) ||
				strings.Contains(strings.ToLower(string(s.SourceTool)), queryLower) ||
				strings.Contains(strings.ToLower(s.Title), queryLower) {
				filtered = append(filtered, s)
			}
		}
		filteredSessions = filtered
	}

	if m.sourceFilter != "" {
		var sourceFiltered []session.Session
		for _, s := range filteredSessions {
			if string(s.SourceTool) == m.sourceFilter {
				sourceFiltered = append(sourceFiltered, s)
			}
		}
		filteredSessions = sourceFiltered
	}

	if m.hiddenManager != nil {
		var visible []session.Session
		for _, s := range filteredSessions {
			if !m.hiddenManager.IsHidden(s.ID) {
				visible = append(visible, s)
			}
		}
		filteredSessions = visible
	}

	return filteredSessions
}

func (m model) View() string {
	filteredSessions := m.getVisibleSessions()

	grouped := groupSessionsByProject(filteredSessions)

	headerHeight := 2 // title + filter row
	if m.searching {
		headerHeight = 3 // title + search row + filter row
	}
	footerHeight := 1

	width := m.width
	height := m.height
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}

	contentHeight := height - headerHeight - footerHeight - 2

	header := headerPanelStyle.Width(width - 2).Render(titleStyle.Render(" Agent Session Manager "))

	if m.searching {
		searchBox := searchBoxStyle.Width(width - 4).Render(
			searchPromptStyle.Render("/ ") + searchInputStyle.Render(m.query),
		)
		header = header + "\n" + searchBox
	}

	// Render filter row
	filterRow := m.renderFilterRow(width)
	header = header + "\n" + filterRow

	var s string
	if m.query != "" {
		s += fmt.Sprintf("Projects (%d sessions):\n\n", len(filteredSessions))
	} else {
		s += fmt.Sprintf("Projects (%d sessions):\n\n", len(filteredSessions))
	}

	type visibleRow struct {
		sessionIdx  int
		isHeader    bool
		projectPath string
	}
	var visibleRows []visibleRow

	for _, group := range grouped {
		visibleRows = append(visibleRows, visibleRow{isHeader: true, projectPath: group.projectPath})

		for i := range filteredSessions {
			if filteredSessions[i].ProjectPath == group.projectPath {
				visibleRows = append(visibleRows, visibleRow{sessionIdx: i, isHeader: false})
			}
		}
	}

	displayRows := visibleRows
	if contentHeight > 0 && len(visibleRows) > contentHeight-2 {
		displayRows = visibleRows[:contentHeight-2]
	}

	for _, row := range displayRows {
		if row.isHeader {
			projectStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#86EFAC")).
				Padding(0, 0)
			s += projectStyle.Render("▸ "+simplifyPath(row.projectPath)) + "\n"
		} else {
			sess := filteredSessions[row.sessionIdx]
			cursor := "  "
			style := itemStyle

			if m.cursor < len(filteredSessions) && filteredSessions[m.cursor].ID == sess.ID {
				cursor = "> "
				style = selectedStyle.Copy().Background(selectedColor)
			}

			sourceTool := sess.SourceTool
			var sourceStyle lipgloss.Style
			switch sourceTool {
			case session.SourceOpenCode:
				sourceStyle = lipgloss.NewStyle().Foreground(openCodeColor)
			case session.SourceClaude:
				sourceStyle = lipgloss.NewStyle().Foreground(claudeColor)
			case session.SourceQwen:
				sourceStyle = lipgloss.NewStyle().Foreground(qwenColor)
			default:
				sourceStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
			}

			sourceStr := string(sourceTool)

			line := fmt.Sprintf("%s%s [%s]",
				cursor,
				sess.Title,
				sourceStyle.Render(sourceStr),
			)
			s += style.Render(line) + "\n"
		}
	}

	sessionPanel := panelStyle.Width(width - 2).Height(contentHeight).Render(s)

	footerHelp := "↑/k up  ↓/j down  Enter attach  q quit"
	if m.searching {
		footerHelp = "type to filter  Esc exit"
	} else if m.showHiddenOverlay {
		footerHelp = "↑/k up  ↓/j down  r restore  a restore all  esc close"
	} else {
		footerHelp += "  1-4 filter  h hide  H hidden"
	}
	footer := footerPanelStyle.Width(width - 2).Render(
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Render(footerHelp),
	)

	view := header + "\n" + sessionPanel + "\n" + footer

	// Render hidden overlay if active
	if m.showHiddenOverlay && m.hiddenManager != nil {
		hiddenIDs := m.hiddenManager.List()
		overlay := m.renderHiddenOverlay(hiddenIDs, width, height)
		view = lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, view) + overlay
	}

	return view
}

func (m model) renderHiddenOverlay(hiddenIDs []string, width, height int) string {
	overlayWidth := width / 2
	if overlayWidth < 40 {
		overlayWidth = 40
	}

	title := " Hidden Sessions "
	content := fmt.Sprintf("Hidden (%d):\n\n", len(hiddenIDs))

	if len(hiddenIDs) == 0 {
		content += "  (no hidden sessions)"
	} else {
		for i, id := range hiddenIDs {
			cursor := "  "
			if i == m.hiddenCursor {
				cursor = "> "
			}
			content += fmt.Sprintf("%s%s\n", cursor, id)
		}
	}

	overlayContent := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FDBA74")).
		Background(bgColor).
		Padding(1, 2).
		Width(overlayWidth).
		Render(titleStyle.Render(title) + "\n\n" + content)

	return overlayContent
}

func (m model) renderFilterRow(width int) string {
	filters := []struct {
		key   string
		value string
		label string
		color lipgloss.Color
	}{
		{"1", "", "All", lipgloss.Color("#FFFFFF")},
		{"2", "claude", "Claude", claudeColor},
		{"3", "opencode", "OpenCode", openCodeColor},
		{"4", "qwen", "Qwen", qwenColor},
	}

	var buttons []string
	for _, f := range filters {
		var btn string
		if m.sourceFilter == f.value {
			btn = filterActiveStyle.Copy().
				BorderForeground(f.color).
				Render(fmt.Sprintf("[%s] %s", f.key, f.label))
		} else {
			btn = filterInactiveStyle.Render(fmt.Sprintf("[%s] %s", f.key, f.label))
		}
		buttons = append(buttons, btn)
	}

	return lipgloss.JoinHorizontal(lipgloss.Center, buttons...)
}

func main() {
	var allSessions []session.Session

	scanners := []struct {
		name    string
		scanner session.Scanner
	}{
		{"OpenCode", session.NewOpenCodeScanner()},
		{"Claude", session.NewClaudeScanner()},
		{"Qwen", session.NewQwenScanner()},
	}

	for _, s := range scanners {
		sessions, err := s.scanner.Scan("")
		if err != nil {
			continue
		}
		allSessions = append(allSessions, sessions...)
	}

	seen := make(map[string]bool)
	var uniqueSessions []session.Session
	for _, sess := range allSessions {
		key := fmt.Sprintf("%s|%s", sess.ID, sess.SourceTool)
		if !seen[key] {
			seen[key] = true
			uniqueSessions = append(uniqueSessions, sess)
		}
	}

	sort.Slice(uniqueSessions, func(i, j int) bool {
		return uniqueSessions[i].LastUpdated > uniqueSessions[j].LastUpdated
	})

	hiddenManager, err := hidden.NewManager("agent-session-manager")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to initialize hidden manager: %v\n", err)
	}

	m := model{
		cursor:            0,
		sessions:          uniqueSessions,
		width:             0,
		height:            0,
		hiddenManager:     hiddenManager,
		showHiddenOverlay: false,
		hiddenCursor:      0,
		sourceFilter:      "",
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
