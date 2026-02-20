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

type focusPanel int

const (
	focusLeft focusPanel = iota
	focusRight
)

type model struct {
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

	// dual-pane navigation
	focusPanel       focusPanel
	projectCursor    int
	sessionCursor    int
	projectScrollOffset  int
	sessionScrollOffset  int
}

type groupedSession struct {
	projectPath string
	sessions    []session.Session
}

func (m model) Init() tea.Cmd {
	return nil
}

// getGroupedProjects returns filtered and grouped projects with their sessions.
func (m model) getGroupedProjects() []groupedSession {
	filtered := m.getFilteredSessions()
	return groupSessionsByProject(filtered)
}

// getSelectedProjectSessions returns sessions for the currently selected project.
func (m model) getSelectedProjectSessions() []session.Session {
	grouped := m.getGroupedProjects()
	if m.projectCursor < 0 || m.projectCursor >= len(grouped) {
		return nil
	}
	return grouped[m.projectCursor].sessions
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

func (m *model) clampProjectCursor() {
	grouped := m.getGroupedProjects()
	if len(grouped) == 0 {
		m.projectCursor = 0
		return
	}
	if m.projectCursor < 0 {
		m.projectCursor = 0
	}
	if m.projectCursor >= len(grouped) {
		m.projectCursor = len(grouped) - 1
	}
}

func (m *model) clampSessionCursor() {
	sessions := m.getSelectedProjectSessions()
	if len(sessions) == 0 {
		m.sessionCursor = 0
		return
	}
	if m.sessionCursor < 0 {
		m.sessionCursor = 0
	}
	if m.sessionCursor >= len(sessions) {
		m.sessionCursor = len(sessions) - 1
	}
}

func (m *model) clearProjectConfirm() {
	m.confirmProjectPath = ""
	m.confirmExpiresAt = time.Time{}
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

		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if m.focusPanel == focusLeft {
				m.projectCursor--
				m.clearProjectConfirm()
			} else {
				m.sessionCursor--
				m.clearProjectConfirm()
			}
		case tea.MouseButtonWheelDown:
			if m.focusPanel == focusLeft {
				m.projectCursor++
				m.clearProjectConfirm()
			} else {
				m.sessionCursor++
				m.clearProjectConfirm()
			}
		}
		m.clampProjectCursor()
		m.clampSessionCursor()
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

		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "1":
			m.sourceFilter = ""
			m.projectCursor = 0
			m.sessionCursor = 0
			m.clearProjectConfirm()
			return m, nil
		case "2":
			m.sourceFilter = "claude"
			m.projectCursor = 0
			m.sessionCursor = 0
			m.clearProjectConfirm()
			return m, nil
		case "3":
			m.sourceFilter = "opencode"
			m.projectCursor = 0
			m.sessionCursor = 0
			m.clearProjectConfirm()
			return m, nil
		case "4":
			m.sourceFilter = "qwen"
			m.projectCursor = 0
			m.sessionCursor = 0
			m.clearProjectConfirm()
			return m, nil
		case "H":
			m.showHiddenOverlay = true
			m.hiddenCursor = 0
			return m, nil
		case "h":
			sessions := m.getSelectedProjectSessions()
			if len(sessions) > 0 && m.sessionCursor < len(sessions) && m.hiddenManager != nil {
				if err := m.hiddenManager.Add(sessions[m.sessionCursor].ID); err != nil {
					fmt.Fprintf(os.Stderr, "failed to hide session: %v\n", err)
				}
			}
			m.clearProjectConfirm()
			return m, nil
		case "tab":
			// Switch between left and right panels
			if m.focusPanel == focusLeft {
				m.focusPanel = focusRight
			} else {
				m.focusPanel = focusLeft
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
				m.projectCursor = 0
				m.sessionCursor = 0
				m.statusMessage = "search cleared"
			}
			m.clearProjectConfirm()
			return m, nil
		case tea.KeyBackspace, tea.KeyCtrlH:
			queryRunes := []rune(m.query)
			if len(queryRunes) > 0 {
				m.query = string(queryRunes[:len(queryRunes)-1])
				m.projectCursor = 0
				m.sessionCursor = 0
			}
			m.clearProjectConfirm()
			return m, nil
		case tea.KeyUp:
			if m.focusPanel == focusLeft {
				m.projectCursor--
			} else {
				m.sessionCursor--
			}
			m.clearProjectConfirm()
			m.clampProjectCursor()
			m.clampSessionCursor()
			return m, nil
		case tea.KeyDown:
			if m.focusPanel == focusLeft {
				m.projectCursor++
			} else {
				m.sessionCursor++
			}
			m.clearProjectConfirm()
			m.clampProjectCursor()
			m.clampSessionCursor()
			return m, nil
		case tea.KeyEnter:
			sessions := m.getSelectedProjectSessions()
			if len(sessions) == 0 || m.sessionCursor >= len(sessions) {
				// No sessions in selected project, try to open shell at project
				grouped := m.getGroupedProjects()
				if len(grouped) > 0 && m.projectCursor < len(grouped) {
					selectedProject := grouped[m.projectCursor].projectPath
					now := time.Now()
					if m.confirmProjectPath == selectedProject && now.Before(m.confirmExpiresAt) {
						m.projectShellPath = selectedProject
						m.clearProjectConfirm()
						return m, tea.Quit
					}
					m.confirmProjectPath = selectedProject
					m.confirmExpiresAt = now.Add(projectConfirmWindow)
					m.statusMessage = "press Enter again within 2s to open shell"
				}
				return m, nil
			}

			selected := sessions[m.sessionCursor]
			launchSession(selected)
			m.clearProjectConfirm()
			return m, nil
		}

		switch msg.String() {
		case "k":
			if m.focusPanel == focusLeft {
				m.projectCursor--
			} else {
				m.sessionCursor--
			}
			m.clearProjectConfirm()
			m.clampProjectCursor()
			m.clampSessionCursor()
			return m, nil
		case "j":
			if m.focusPanel == focusLeft {
				m.projectCursor++
			} else {
				m.sessionCursor++
			}
			m.clearProjectConfirm()
			m.clampProjectCursor()
			m.clampSessionCursor()
			return m, nil
		}

		if typed := keyToText(msg); typed != "" {
			m.query += typed
			m.projectCursor = 0
			m.sessionCursor = 0
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

	footerText := "Tab switch  ↑/k move  ↓/j move  Enter open  Esc clear search  q quit  1-4 filter  h hide  H hidden"
	if m.query != "" {
		footerText = "SEARCH  |  " + normalizeSingleLine(m.query) + "  |  Tab switch  ↑/k move  ↓/j move  Enter open  Esc clear search"
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

	// Split width into left and right panels
	leftPanelWidth := width / 3
	if leftPanelWidth < 20 {
		leftPanelWidth = 20
	}
	rightPanelWidth := width - leftPanelWidth - 1 // -1 for separator
	if rightPanelWidth < 30 {
		rightPanelWidth = 30
	}

	leftInnerWidth := leftPanelWidth - panelStyle.GetHorizontalFrameSize()
	rightInnerWidth := rightPanelWidth - panelStyle.GetHorizontalFrameSize()

	// Build left panel (projects)
	grouped := m.getGroupedProjects()
	var leftBody strings.Builder
	leftBody.WriteString(fmt.Sprintf("Projects (%d)\n\n", len(grouped)))

	leftVisibleRows := contentHeight - 3
	if leftVisibleRows < 1 {
		leftVisibleRows = 1
	}
	leftStart, leftEnd := getVisibleWindow(len(grouped), m.projectCursor, leftVisibleRows)

	for i := leftStart; i < leftEnd; i++ {
		group := grouped[i]
		isSelected := i == m.projectCursor
		isFocused := m.focusPanel == focusLeft

		pathText := truncateRunesNoEllipsis(simplifyPath(group.projectPath), leftInnerWidth-2)
		sessionCount := fmt.Sprintf(" (%d)", len(group.sessions))
		countWidth := runewidth.StringWidth(sessionCount)
		maxPath := leftInnerWidth - countWidth - 2
		if maxPath < 5 {
			maxPath = 5
		}
		pathText = truncateRunesNoEllipsis(pathText, maxPath)
		line := "  " + pathText + sessionCount

		if isSelected {
			if isFocused {
				line = "> " + pathText + sessionCount
				line = padRightWidth(truncateRunesNoEllipsis(line, leftInnerWidth), leftInnerWidth)
				leftBody.WriteString(selectedStyle.Render(line) + "\n")
			} else {
				line = "▸ " + pathText + sessionCount
				line = padRightWidth(truncateRunesNoEllipsis(line, leftInnerWidth), leftInnerWidth)
				leftBody.WriteString(projectStyle.Render(line) + "\n")
			}
		} else {
			line = padRightWidth(truncateRunesNoEllipsis(line, leftInnerWidth), leftInnerWidth)
			leftBody.WriteString(itemStyle.Render(line) + "\n")
		}
	}

	if len(grouped) == 0 {
		leftBody.WriteString("  (no projects)\n")
	}

	leftPanel := panelStyle.Width(leftPanelWidth).Height(contentHeight).Render(leftBody.String())

	// Build right panel (sessions for selected project)
	var rightBody strings.Builder
	var selectedProjectPath string
	if len(grouped) > 0 && m.projectCursor < len(grouped) {
		selectedProjectPath = grouped[m.projectCursor].projectPath
		rightBody.WriteString(fmt.Sprintf("Sessions: %s\n\n", simplifyPath(selectedProjectPath)))
	} else {
		rightBody.WriteString("Sessions\n\n")
	}

	sessions := m.getSelectedProjectSessions()
	rightVisibleRows := contentHeight - 3
	if rightVisibleRows < 1 {
		rightVisibleRows = 1
	}
	rightStart, rightEnd := getVisibleWindow(len(sessions), m.sessionCursor, rightVisibleRows)

	for i := rightStart; i < rightEnd; i++ {
		sess := sessions[i]
		isSelected := i == m.sessionCursor
		isFocused := m.focusPanel == focusRight

		sourceText := string(sess.SourceTool)
		sourceStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7A7A7A")).
			Faint(true)
		switch sess.SourceTool {
		case session.SourceOpenCode:
			sourceStyle = lipgloss.NewStyle().Foreground(openCodeColor).Faint(true)
		case session.SourceClaude:
			sourceStyle = lipgloss.NewStyle().Foreground(claudeColor).Faint(true)
		case session.SourceQwen:
			sourceStyle = lipgloss.NewStyle().Foreground(qwenColor).Faint(true)
		}

		badgePlain := sourceText
		badgeWidth := runewidth.StringWidth(badgePlain)
		maxTitle := rightInnerWidth - badgeWidth - 3
		if maxTitle < 8 {
			maxTitle = 8
		}
		title := truncateRunesNoEllipsis(normalizeSingleLine(sess.Title), maxTitle)
		badge := sourceStyle.Render(badgePlain)
		leftWidth := rightInnerWidth - badgeWidth
		if leftWidth < 3 {
			leftWidth = 3
		}
		line := "  " + title + " " + badge

		if isSelected {
			if isFocused {
				selectedText := "> " + title + " " + badgePlain
				selectedText = padRightWidth(truncateRunesNoEllipsis(selectedText, rightInnerWidth), rightInnerWidth)
				rightBody.WriteString(selectedStyle.Render(selectedText) + "\n")
			} else {
				line = "▸ " + title + " " + badgePlain
				line = padRightWidth(truncateRunesNoEllipsis(line, rightInnerWidth), rightInnerWidth)
				rightBody.WriteString(projectStyle.Render(line) + "\n")
			}
		} else {
			line = padRightWidth(truncateRunesNoEllipsis(line, rightInnerWidth), rightInnerWidth)
			rightBody.WriteString(itemStyle.Render(line) + "\n")
		}
	}

	if len(sessions) == 0 {
		if selectedProjectPath != "" {
			rightBody.WriteString("  (no sessions in this project)\n")
		} else {
			rightBody.WriteString("  (select a project)\n")
		}
	}

	rightPanel := panelStyle.Width(rightPanelWidth).Height(contentHeight).Render(rightBody.String())

	// Join panels horizontally
	view := header + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel) + "\n" + footer

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
		sessions:          collectSessions(),
		hiddenManager:     hiddenManager,
		showHiddenOverlay: false,
		hiddenCursor:      0,
		sourceFilter:      "",
		focusPanel:        focusLeft,
		projectCursor:     0,
		sessionCursor:     0,
		projectScrollOffset: 0,
		sessionScrollOffset: 0,
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
