package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"agent-session-manager/internal/hidden"
	"agent-session-manager/session"
)

var (
	openCodeColor = lipgloss.Color("#86EFAC")
	claudeColor   = lipgloss.Color("#FDBA74")
	qwenColor     = lipgloss.Color("#93C5FD")

	bgColor       = lipgloss.Color("#1E1E1E")
	borderColor   = lipgloss.Color("#3C3C3C")
	textColor     = lipgloss.Color("#FAFAFA")
	selectedColor = lipgloss.Color("#569CD6")
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#334155")).
			Padding(0, 1)

	itemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	projectStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(openCodeColor)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(selectedColor)

	panelStyle = lipgloss.NewStyle().
			Background(bgColor).
			Padding(1, 1)

	footerPanelStyle = lipgloss.NewStyle().
				Background(bgColor).
				Padding(0, 1)

	searchLineStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#1D4ED8")).
			Padding(0, 1)

	filterActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(selectedColor).
				Padding(0, 1)

	filterInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				Padding(0, 1)
)

const projectConfirmWindow = 2 * time.Second

type rowKind int

const (
	rowProject rowKind = iota
	rowSession
)

type listRow struct {
	kind        rowKind
	projectPath string
	session     session.Session
}

type model struct {
	cursorRow         int
	sessions          []session.Session
	query             string
	width             int
	height            int
	hiddenManager     *hidden.Manager
	showHiddenOverlay bool
	hiddenCursor      int
	sourceFilter      string // "", "opencode", "claude", "qwen"

	confirmProjectPath string
	confirmExpiresAt   time.Time
	projectShellPath   string
	statusMessage      string
}

type groupedSession struct {
	projectPath string
	sessions    []session.Session
}

func (m model) Init() tea.Cmd {
	return nil
}

// keyToText extracts typed text from a key message across terminal variants.
func keyToText(msg tea.KeyMsg) string {
	if len(msg.Runes) > 0 {
		return string(msg.Runes)
	}

	switch msg.Type {
	case tea.KeySpace:
		return " "
	default:
		return ""
	}
}

// groupSessionsByProject groups sessions by project and sorts projects by latest session time.
func groupSessionsByProject(sessions []session.Session) []groupedSession {
	groups := make(map[string][]session.Session)
	for _, sess := range sessions {
		groups[sess.ProjectPath] = append(groups[sess.ProjectPath], sess)
	}

	var result []groupedSession
	for path, sessList := range groups {
		sort.Slice(sessList, func(i, j int) bool {
			return sessList[i].LastUpdated > sessList[j].LastUpdated
		})
		result = append(result, groupedSession{
			projectPath: path,
			sessions:    sessList,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		var iTime, jTime int64
		if len(result[i].sessions) > 0 {
			iTime = result[i].sessions[0].LastUpdated
		}
		if len(result[j].sessions) > 0 {
			jTime = result[j].sessions[0].LastUpdated
		}
		return iTime > jTime
	})

	return result
}

// simplifyPath returns a project path with home directory replaced by ~.
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

func truncateRunes(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= maxLen {
		return s
	}
	return runewidth.Truncate(s, maxLen, "…")
}

func truncateRunesNoEllipsis(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= maxLen {
		return s
	}
	return runewidth.Truncate(s, maxLen, "")
}

func padRightWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	current := runewidth.StringWidth(s)
	if current >= width {
		return s
	}
	return s + strings.Repeat(" ", width-current)
}

// normalizeSingleLine ensures text is rendered as one terminal line.
func normalizeSingleLine(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}

func (m model) getFilteredSessions() []session.Session {
	filtered := m.sessions

	if m.query != "" {
		queryLower := strings.ToLower(m.query)
		var byTitle []session.Session
		for _, s := range filtered {
			title := normalizeSingleLine(s.Title)
			if strings.Contains(strings.ToLower(title), queryLower) {
				byTitle = append(byTitle, s)
			}
		}
		filtered = byTitle
	}

	if m.sourceFilter != "" {
		var bySource []session.Session
		for _, s := range filtered {
			if string(s.SourceTool) == m.sourceFilter {
				bySource = append(bySource, s)
			}
		}
		filtered = bySource
	}

	if m.hiddenManager != nil {
		var visible []session.Session
		for _, s := range filtered {
			if !m.hiddenManager.IsHidden(s.ID) {
				visible = append(visible, s)
			}
		}
		filtered = visible
	}

	return filtered
}

func (m model) buildRows() []listRow {
	filteredSessions := m.getFilteredSessions()
	grouped := groupSessionsByProject(filteredSessions)

	var rows []listRow
	for _, group := range grouped {
		rows = append(rows, listRow{
			kind:        rowProject,
			projectPath: group.projectPath,
		})
		for _, sess := range group.sessions {
			rows = append(rows, listRow{
				kind:        rowSession,
				projectPath: group.projectPath,
				session:     sess,
			})
		}
	}

	return rows
}

func (m *model) clampCursor(rows []listRow) {
	if len(rows) == 0 {
		m.cursorRow = 0
		return
	}
	if m.cursorRow < 0 {
		m.cursorRow = 0
	}
	if m.cursorRow >= len(rows) {
		m.cursorRow = len(rows) - 1
	}
}

func (m *model) clearProjectConfirm() {
	m.confirmProjectPath = ""
	m.confirmExpiresAt = time.Time{}
}

func (m *model) moveCursor(delta int, rows []listRow) {
	if len(rows) == 0 {
		m.cursorRow = 0
		return
	}

	m.cursorRow += delta
	if m.cursorRow < 0 {
		m.cursorRow = 0
	}
	if m.cursorRow >= len(rows) {
		m.cursorRow = len(rows) - 1
	}
}

func launchSession(sess session.Session) {
	var restoreCmd *exec.Cmd
	switch sess.SourceTool {
	case session.SourceClaude:
		restoreCmd = exec.Command("claude", "-r", sess.ID)
	case session.SourceOpenCode:
		restoreCmd = exec.Command("opencode", "-s", sess.ID)
	case session.SourceQwen:
		restoreCmd = exec.Command("qwen", "-r", sess.ID)
	default:
		return
	}

	if err := restoreCmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to launch %s: %v\n", sess.SourceTool, err)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.MouseMsg:
		if m.showHiddenOverlay {
			return m, nil
		}

		rows := m.buildRows()
		m.clampCursor(rows)

		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.moveCursor(-1, rows)
			m.clearProjectConfirm()
		case tea.MouseButtonWheelDown:
			m.moveCursor(1, rows)
			m.clearProjectConfirm()
		}
		return m, nil
	case tea.KeyMsg:
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
				if m.hiddenCursor < len(hiddenIDs) && m.hiddenManager != nil {
					if err := m.hiddenManager.Remove(hiddenIDs[m.hiddenCursor]); err != nil {
						fmt.Fprintf(os.Stderr, "failed to restore session: %v\n", err)
					}
				}
			case "a":
				if m.hiddenManager != nil {
					for _, id := range hiddenIDs {
						if err := m.hiddenManager.Remove(id); err != nil {
							fmt.Fprintf(os.Stderr, "failed to restore session %s: %v\n", id, err)
						}
					}
				}
				m.showHiddenOverlay = false
				m.hiddenCursor = 0
			}
			return m, nil
		}

		rows := m.buildRows()
		m.clampCursor(rows)

		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "1":
			m.sourceFilter = ""
			m.cursorRow = 0
			m.clearProjectConfirm()
			return m, nil
		case "2":
			m.sourceFilter = "claude"
			m.cursorRow = 0
			m.clearProjectConfirm()
			return m, nil
		case "3":
			m.sourceFilter = "opencode"
			m.cursorRow = 0
			m.clearProjectConfirm()
			return m, nil
		case "4":
			m.sourceFilter = "qwen"
			m.cursorRow = 0
			m.clearProjectConfirm()
			return m, nil
		case "H":
			m.showHiddenOverlay = true
			m.hiddenCursor = 0
			return m, nil
		case "h":
			if len(rows) > 0 && m.cursorRow < len(rows) && rows[m.cursorRow].kind == rowSession && m.hiddenManager != nil {
				if err := m.hiddenManager.Add(rows[m.cursorRow].session.ID); err != nil {
					fmt.Fprintf(os.Stderr, "failed to hide session: %v\n", err)
				}
			}
			m.clearProjectConfirm()
			return m, nil
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			if m.query != "" {
				m.query = ""
				m.cursorRow = 0
				m.statusMessage = "search cleared"
			}
			m.clearProjectConfirm()
			return m, nil
		case tea.KeyBackspace, tea.KeyCtrlH:
			queryRunes := []rune(m.query)
			if len(queryRunes) > 0 {
				m.query = string(queryRunes[:len(queryRunes)-1])
				m.cursorRow = 0
			}
			m.clearProjectConfirm()
			return m, nil
		case tea.KeyUp:
			m.moveCursor(-1, rows)
			m.clearProjectConfirm()
			return m, nil
		case tea.KeyDown:
			m.moveCursor(1, rows)
			m.clearProjectConfirm()
			return m, nil
		case tea.KeyEnter:
			if len(rows) == 0 || m.cursorRow >= len(rows) {
				return m, nil
			}

			selected := rows[m.cursorRow]
			if selected.kind == rowSession {
				launchSession(selected.session)
				m.clearProjectConfirm()
				return m, nil
			}

			now := time.Now()
			if m.confirmProjectPath == selected.projectPath && now.Before(m.confirmExpiresAt) {
				m.projectShellPath = selected.projectPath
				m.clearProjectConfirm()
				return m, tea.Quit
			}

			m.confirmProjectPath = selected.projectPath
			m.confirmExpiresAt = now.Add(projectConfirmWindow)
			m.statusMessage = "press Enter again within 2s to open shell"
			return m, nil
		}

		switch msg.String() {
		case "k":
			m.moveCursor(-1, rows)
			m.clearProjectConfirm()
			return m, nil
		case "j":
			m.moveCursor(1, rows)
			m.clearProjectConfirm()
			return m, nil
		}

		if typed := keyToText(msg); typed != "" {
			m.query += typed
			m.cursorRow = 0
			m.clearProjectConfirm()
			m.statusMessage = ""
			return m, nil
		}
	}

	return m, nil
}

func getVisibleWindow(total, cursor, maxRows int) (int, int) {
	if total <= 0 || maxRows <= 0 {
		return 0, 0
	}

	if total <= maxRows {
		return 0, total
	}

	start := 0
	if cursor >= maxRows {
		start = cursor - maxRows + 1
	}
	if start+maxRows > total {
		start = total - maxRows
	}
	end := start + maxRows
	return start, end
}

func (m model) View() string {
	rows := m.buildRows()

	cursor := m.cursorRow
	if len(rows) == 0 {
		cursor = 0
	} else {
		if cursor < 0 {
			cursor = 0
		}
		if cursor >= len(rows) {
			cursor = len(rows) - 1
		}
	}

	width := m.width
	height := m.height
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}

	searchValue := normalizeSingleLine(m.query)
	if searchValue == "" {
		searchValue = "Type to search"
	}
	searchLine := searchLineStyle.Width(width - 2).Render("SEARCH> " + truncateRunes(searchValue, width-12))

	header := strings.Join([]string{
		titleStyle.Render(" Agent Session Manager "),
		searchLine,
		m.renderFilterRow(),
	}, "\n")

	footerText := "NORMAL  |  ↑/k move  ↓/j move  Enter open  Esc clear search  q quit  1-4 filter  h hide  H hidden"
	if m.query != "" {
		footerText = "SEARCH  |  " + normalizeSingleLine(m.query) + "  |  ↑/k move  ↓/j move  Enter open  Esc clear search"
	}
	if m.confirmProjectPath != "" && time.Now().Before(m.confirmExpiresAt) {
		footerText = fmt.Sprintf("CONFIRM  |  Press Enter again within 2s to open shell at %s", simplifyPath(m.confirmProjectPath))
	}
	if m.statusMessage != "" && m.confirmProjectPath == "" {
		footerText = strings.ToUpper(m.statusMessage) + "  |  " + footerText
	}
	footerText = truncateRunes(normalizeSingleLine(footerText), width-4)

	footer := footerPanelStyle.Width(width - 2).Render(
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Render(footerText),
	)

	headerHeight := lipgloss.Height(header)
	footerHeight := lipgloss.Height(footer)
	contentHeight := height - headerHeight - footerHeight
	if contentHeight < 3 {
		contentHeight = 3
	}

	var body strings.Builder
	filteredSessions := m.getFilteredSessions()
	body.WriteString(fmt.Sprintf("Projects (%d sessions):\n\n", len(filteredSessions)))

	visibleRows := contentHeight - 2
	if visibleRows < 1 {
		visibleRows = 1
	}
	start, end := getVisibleWindow(len(rows), cursor, visibleRows)
	panelWidth := width - 2
	if panelWidth < 20 {
		panelWidth = 20
	}
	lineWidth := panelWidth - panelStyle.GetHorizontalFrameSize()
	if lineWidth < 20 {
		lineWidth = 20
	}

	for i := start; i < end; i++ {
		row := rows[i]
		isSelected := i == cursor

		switch row.kind {
		case rowProject:
			pathText := truncateRunesNoEllipsis(simplifyPath(row.projectPath), lineWidth-2)
			line := "▸ " + pathText
			line = padRightWidth(truncateRunesNoEllipsis(line, lineWidth), lineWidth)
			if isSelected {
				line = "> " + truncateRunesNoEllipsis(pathText, lineWidth-2)
				line = padRightWidth(truncateRunesNoEllipsis(line, lineWidth), lineWidth)
				body.WriteString(selectedStyle.Render(line) + "\n")
			} else {
				body.WriteString(projectStyle.Render(line) + "\n")
			}
		case rowSession:
			sourceText := string(row.session.SourceTool)
			sourceStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7A7A7A")).
				Faint(true)
			switch row.session.SourceTool {
			case session.SourceOpenCode:
				sourceStyle = lipgloss.NewStyle().Foreground(openCodeColor).Faint(true)
			case session.SourceClaude:
				sourceStyle = lipgloss.NewStyle().Foreground(claudeColor).Faint(true)
			case session.SourceQwen:
				sourceStyle = lipgloss.NewStyle().Foreground(qwenColor).Faint(true)
			}

			badgePlain := sourceText
			badgeWidth := runewidth.StringWidth(badgePlain)
			maxTitle := lineWidth - badgeWidth - 3
			if maxTitle < 8 {
				maxTitle = 8
			}
			title := truncateRunesNoEllipsis(normalizeSingleLine(row.session.Title), maxTitle)
			badge := sourceStyle.Render(badgePlain)
			leftWidth := lineWidth - badgeWidth
			if leftWidth < 3 {
				leftWidth = 3
			}
			left := padRightWidth(truncateRunesNoEllipsis("  "+title+" ", leftWidth), leftWidth)
			line := left + badge
			if isSelected {
				selectedText := fmt.Sprintf("> %s %s", title, badgePlain)
				selectedText = padRightWidth(truncateRunesNoEllipsis(selectedText, lineWidth), lineWidth)
				body.WriteString(selectedStyle.Render(selectedText) + "\n")
			} else {
				body.WriteString(itemStyle.Render(line) + "\n")
			}
		}
	}

	if len(rows) == 0 {
		body.WriteString("  (no sessions match current filters)\n")
	}

	sessionPanel := panelStyle.Width(panelWidth).Height(contentHeight).MaxHeight(contentHeight).Render(body.String())

	view := header + "\n" + sessionPanel + "\n" + footer

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

func (m model) renderFilterRow() string {
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

	return lipgloss.JoinHorizontal(lipgloss.Left, buttons...)
}

func collectSessions() []session.Session {
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

	return uniqueSessions
}

func runShellAtPath(projectPath string) error {
	if projectPath == "" {
		return nil
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	cmd := exec.Command(shell)
	cmd.Dir = projectPath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func main() {
	hiddenManager, err := hidden.NewManager("agent-session-manager")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to initialize hidden manager: %v\n", err)
	}

	m := model{
		cursorRow:         0,
		sessions:          collectSessions(),
		hiddenManager:     hiddenManager,
		showHiddenOverlay: false,
		hiddenCursor:      0,
		sourceFilter:      "",
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if fm, ok := finalModel.(model); ok && fm.projectShellPath != "" {
		if err := runShellAtPath(fm.projectShellPath); err != nil {
			fmt.Fprintf(os.Stderr, "failed to open project shell: %v\n", err)
			os.Exit(1)
		}
	}
}
