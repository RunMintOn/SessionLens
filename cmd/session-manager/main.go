package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"agent-session-manager/internal/hidden"
	"agent-session-manager/session"
)

type launcherConfig struct {
	TerminalCmd        string
	WSLEntryCmd        string
	WSLShell           string
	RestoreCmdClaude   string
	RestoreCmdOpenCode string
	RestoreCmdQwen     string
	RestoreCmdCodex    string
}

var launcherCfg = loadLauncherConfigFromEnv()

func envOrDefault(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func loadLauncherConfigFromEnv() launcherConfig {
	return launcherConfig{
		TerminalCmd:        envOrDefault("ASM_TERMINAL_CMD", "wt"),
		WSLEntryCmd:        envOrDefault("ASM_WSL_ENTRY_CMD", "wsl.exe"),
		WSLShell:           envOrDefault("ASM_WSL_SHELL", "zsh -lic"),
		RestoreCmdClaude:   envOrDefault("ASM_RESTORE_CMD_CLAUDE", "claude -r {id}"),
		RestoreCmdOpenCode: envOrDefault("ASM_RESTORE_CMD_OPENCODE", "opencode -s {id}"),
		RestoreCmdQwen:     envOrDefault("ASM_RESTORE_CMD_QWEN", "qwen -r {id}"),
		RestoreCmdCodex:    envOrDefault("ASM_RESTORE_CMD_CODEX", "codex resume {id}"),
	}
}

var (
	openCodeColor = lipgloss.Color("#86EFAC")
	claudeColor   = lipgloss.Color("#FDBA74")
	qwenColor     = lipgloss.Color("#93C5FD")
	codexColor    = lipgloss.Color("#FCA5A5")

	bgColor             = lipgloss.Color("#1E1E1E")
	borderColor         = lipgloss.Color("#3C3C3C")
	textColor           = lipgloss.Color("#FAFAFA")
	selectedColor       = lipgloss.Color("#569CD6")
	leftMutedColor      = lipgloss.Color("#94A3B8")
	leftCardColor       = lipgloss.Color("#20262D")
	leftCardBorderColor = lipgloss.Color("#2F3944")
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

const (
	projectConfirmWindow = 2 * time.Second
	leftPanelRatio       = 0.40
	leftPanelMinWidth    = 28
	rightPanelMinWidth   = 30
	projectCardRowHeight = 3 // title + meta + spacer
)

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
	searchActive      bool
	width             int
	height            int
	hiddenManager     *hidden.Manager
	showHiddenOverlay bool
	hiddenCursor      int
	sourceFilter      string // "", "opencode", "claude", "qwen", "codex"

	confirmProjectPath string
	confirmExpiresAt   time.Time
	projectShellPath   string
	statusMessage      string

	// dual-pane navigation
	focusPanel          focusPanel
	projectCursor       int
	sessionCursor       int
	projectScrollOffset int
	sessionScrollOffset int
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
	truncated := runewidth.Truncate(s, maxLen, "…")
	// 确保截断后是有效的 UTF-8，避免显示乱码
	if !utf8.ValidString(truncated) {
		// 如果无效，回退一个字符
		runes := []rune(s)
		if len(runes) > 0 {
			truncated = string(runes[:len(runes)-1])
			truncated = runewidth.Truncate(truncated, maxLen, "…")
		}
	}
	return truncated
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

func shellQuoteSingle(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func restoreTemplateForSession(sess session.Session, cfg launcherConfig) (string, error) {
	switch sess.SourceTool {
	case session.SourceClaude:
		return cfg.RestoreCmdClaude, nil
	case session.SourceOpenCode:
		return cfg.RestoreCmdOpenCode, nil
	case session.SourceQwen:
		return cfg.RestoreCmdQwen, nil
	case session.SourceCodex:
		return cfg.RestoreCmdCodex, nil
	default:
		return "", fmt.Errorf("unsupported source tool: %s", sess.SourceTool)
	}
}

func renderTemplate(template string, values map[string]string) string {
	rendered := template
	for key, value := range values {
		rendered = strings.ReplaceAll(rendered, "{"+key+"}", value)
	}
	return rendered
}

func shouldUseWSL(projectPath string) bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	if strings.HasPrefix(projectPath, "/") {
		return true
	}
	return false
}

func commandBinary(command string) (string, error) {
	if strings.Contains(command, "/") || strings.Contains(command, "\\") {
		if info, err := os.Stat(command); err == nil && !info.IsDir() {
			return command, nil
		}
		return "", fmt.Errorf("%s not found", command)
	}
	if path, err := exec.LookPath(command); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("%s not found in PATH", command)
}

func windowsTerminalBinary(cfg launcherConfig) (string, error) {
	if path, err := commandBinary(cfg.TerminalCmd); err == nil {
		return path, nil
	}

	candidates, err := filepath.Glob("/mnt/c/Users/*/AppData/Local/Microsoft/WindowsApps/wt.exe")
	if err == nil {
		for _, candidate := range candidates {
			if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
				return candidate, nil
			}
		}
	}

	return "", fmt.Errorf("wt not found (windows terminal not discoverable from wsl)")
}

func windowsInteropProbeBinary() string {
	if cmdBin, err := exec.LookPath("cmd.exe"); err == nil {
		return cmdBin
	}
	candidate := "/mnt/c/Windows/System32/cmd.exe"
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate
	}
	return ""
}

func ensureWindowsInteropAvailable() error {
	if os.Getenv("WSL_DISTRO_NAME") == "" {
		return nil
	}

	cmdBin := windowsInteropProbeBinary()
	if cmdBin == "" {
		return fmt.Errorf("windows interop unavailable (cmd.exe not found)")
	}

	out, err := exec.Command(cmdBin, "/C", "echo", "ok").CombinedOutput()
	output := strings.TrimSpace(string(out))
	if strings.Contains(output, "UtilBindVsockAnyPort") {
		return fmt.Errorf("windows interop unavailable in this wsl session")
	}
	if err != nil {
		return fmt.Errorf("windows interop unavailable: %w", err)
	}
	return nil
}

func resolveRestoreScript(sess session.Session, cfg launcherConfig) (string, error) {
	projectPath := filepath.Clean(sess.ProjectPath)
	template, err := restoreTemplateForSession(sess, cfg)
	if err != nil {
		return "", err
	}
	replaceValues := map[string]string{
		"id":      shellQuoteSingle(sess.ID),
		"project": shellQuoteSingle(projectPath),
	}
	restoreCmd := renderTemplate(template, replaceValues)
	if strings.Contains(restoreCmd, "{") || strings.Contains(restoreCmd, "}") {
		return "", fmt.Errorf("invalid restore command template: unresolved placeholder")
	}
	return "cd " + shellQuoteSingle(projectPath) + " && " + restoreCmd, nil
}

func wslShellArgs(cfg launcherConfig) ([]string, error) {
	parts := strings.Fields(cfg.WSLShell)
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid WSL shell command")
	}
	return parts, nil
}

func launchSession(sess session.Session) error {
	if strings.TrimSpace(sess.ProjectPath) == "" {
		return fmt.Errorf("missing project path for session %s", sess.ID)
	}

	script, err := resolveRestoreScript(sess, launcherCfg)
	if err != nil {
		return err
	}

	projectPath := filepath.Clean(sess.ProjectPath)

	wtBin, err := windowsTerminalBinary(launcherCfg)
	if err != nil {
		return err
	}
	if err := ensureWindowsInteropAvailable(); err != nil {
		return err
	}

	var wtCmd *exec.Cmd
	if shouldUseWSL(projectPath) {
		wslShell, err := wslShellArgs(launcherCfg)
		if err != nil {
			return err
		}
		args := []string{launcherCfg.WSLEntryCmd, "-e"}
		args = append(args, wslShell...)
		args = append(args, script)
		wtCmd = exec.Command(wtBin, args...)
	} else {
		wtCmd = exec.Command(wtBin, "sh", "-lc", script)
	}
	if err := wtCmd.Start(); err != nil {
		return fmt.Errorf("failed to start %s: %w", wtBin, err)
	}
	return nil
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
			hiddenEntries := []hidden.HiddenEntry{}
			if m.hiddenManager != nil {
				hiddenEntries = m.hiddenManager.List()
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
				if len(hiddenEntries) > 0 && m.hiddenCursor < len(hiddenEntries)-1 {
					m.hiddenCursor++
				}
			case "r":
				if m.hiddenCursor < len(hiddenEntries) && m.hiddenManager != nil {
					if err := m.hiddenManager.Remove(hiddenEntries[m.hiddenCursor].ID); err != nil {
						fmt.Fprintf(os.Stderr, "failed to restore session: %v\n", err)
					}
				}
			case "a":
				if m.hiddenManager != nil {
					for _, entry := range hiddenEntries {
						if err := m.hiddenManager.Remove(entry.ID); err != nil {
							fmt.Fprintf(os.Stderr, "failed to restore session %s: %v\n", entry.ID, err)
						}
					}
				}
				m.showHiddenOverlay = false
				m.hiddenCursor = 0
			}
			return m, nil
		}

		if m.searchActive {
			switch msg.Type {
			case tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyEsc:
				m.searchActive = false
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
			}

			if typed := keyToText(msg); typed != "" {
				m.query += typed
				m.projectCursor = 0
				m.sessionCursor = 0
				m.clearProjectConfirm()
				m.statusMessage = ""
				return m, nil
			}

			return m, nil
		}

		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "/":
			m.searchActive = true
			m.statusMessage = ""
			m.clearProjectConfirm()
			return m, nil
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
		case "5":
			m.sourceFilter = "codex"
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
				sess := sessions[m.sessionCursor]
				if err := m.hiddenManager.Add(sess.ID, sess.Title); err != nil {
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
		case "right":
			m.focusPanel = focusRight
			m.clearProjectConfirm()
			return m, nil
		case "left":
			m.focusPanel = focusLeft
			m.clearProjectConfirm()
			return m, nil
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
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
			if m.focusPanel == focusLeft {
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

			sessions := m.getSelectedProjectSessions()
			if len(sessions) == 0 || m.sessionCursor >= len(sessions) {
				m.clearProjectConfirm()
				return m, nil
			}

			selected := sessions[m.sessionCursor]
			if err := launchSession(selected); err != nil {
				m.statusMessage = err.Error()
				return m, nil
			}
			m.statusMessage = ""
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

func renderProjectCard(path string, sessionCount, innerWidth int, isSelected, isFocused bool) string {
	if innerWidth < 8 {
		return ""
	}

	pathText := simplifyPath(path)
	titlePrefix := "  "
	if isSelected && isFocused {
		titlePrefix = "> "
	} else if isSelected {
		titlePrefix = "▸ "
	}

	titleWidth := innerWidth - 4
	if titleWidth < 4 {
		titleWidth = 4
	}
	title := truncateRunesNoEllipsis(titlePrefix+pathText, titleWidth)
	meta := truncateRunesNoEllipsis(fmt.Sprintf("  %d sessions", sessionCount), titleWidth)

	content := title + "\n" + lipgloss.NewStyle().Foreground(leftMutedColor).Render(meta)

	base := lipgloss.NewStyle().
		Width(innerWidth).
		Padding(0, 1).
		Foreground(textColor).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(leftCardBorderColor).
		Background(leftCardColor)

	if isSelected && isFocused {
		base = base.BorderForeground(selectedColor).Background(lipgloss.Color("#253343"))
	} else if isSelected {
		base = base.BorderForeground(openCodeColor).Background(lipgloss.Color("#222C35"))
	}

	return base.Render(content)
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
	if searchValue == "" && !m.searchActive {
		searchValue = "/ to search"
	}
	searchLine := searchLineStyle.Width(width - 2).Render("SEARCH> " + truncateRunes(searchValue, width-12))

	header := strings.Join([]string{
		titleStyle.Render(" Agent Session Manager "),
		searchLine,
		m.renderFilterRow(),
	}, "\n")

	footerText := "←/→ switch  ↑/k move  ↓/j move  Enter: Left project / Right resume  / search  q quit  1-5 filter  h hide  H hidden"
	if m.searchActive {
		footerText = "SEARCH INPUT  |  type to search  |  Esc clear & exit search"
	} else if m.query != "" {
		footerText = "SEARCH FILTER  |  " + normalizeSingleLine(m.query) + "  |  / edit  Esc keeps filter"
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
	leftPanelWidth := int(float64(width) * leftPanelRatio)
	if leftPanelWidth < leftPanelMinWidth {
		leftPanelWidth = leftPanelMinWidth
	}
	maxLeftWidth := width - rightPanelMinWidth - 1
	if maxLeftWidth < 20 {
		maxLeftWidth = 20
	}
	if leftPanelWidth > maxLeftWidth {
		leftPanelWidth = maxLeftWidth
	}

	rightPanelWidth := width - leftPanelWidth - 1 // -1 for separator
	if rightPanelWidth < rightPanelMinWidth {
		rightPanelWidth = rightPanelMinWidth
		leftPanelWidth = width - rightPanelWidth - 1
	}
	if leftPanelWidth < 20 {
		leftPanelWidth = 20
		rightPanelWidth = width - leftPanelWidth - 1
	}
	if rightPanelWidth < 20 {
		rightPanelWidth = 20
	}

	leftInnerWidth := leftPanelWidth - panelStyle.GetHorizontalFrameSize()
	rightInnerWidth := rightPanelWidth - panelStyle.GetHorizontalFrameSize()

	// Build left panel (projects)
	grouped := m.getGroupedProjects()
	var leftBody strings.Builder
	leftBody.WriteString(
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E2E8F0")).Render("PROJECTS") + "\n",
	)
	leftBody.WriteString(
		lipgloss.NewStyle().Foreground(leftMutedColor).Render(fmt.Sprintf("%d total", len(grouped))) + "\n\n",
	)

	leftHeaderHeight := 3
	leftVisibleCards := (contentHeight - leftHeaderHeight) / projectCardRowHeight
	if leftVisibleCards < 1 {
		leftVisibleCards = 1
	}
	leftStart, leftEnd := getVisibleWindow(len(grouped), m.projectCursor, leftVisibleCards)
	if leftStart > 0 {
		leftBody.WriteString(lipgloss.NewStyle().Foreground(leftMutedColor).Render("  ↑ more") + "\n")
	}

	for i := leftStart; i < leftEnd; i++ {
		group := grouped[i]
		isSelected := i == m.projectCursor
		isFocused := m.focusPanel == focusLeft
		leftBody.WriteString(renderProjectCard(group.projectPath, len(group.sessions), leftInnerWidth, isSelected, isFocused))
		leftBody.WriteString("\n\n")
	}

	if len(grouped) == 0 {
		leftBody.WriteString(lipgloss.NewStyle().Foreground(leftMutedColor).Render("  (no projects)") + "\n")
	}
	if leftEnd < len(grouped) {
		leftBody.WriteString(lipgloss.NewStyle().Foreground(leftMutedColor).Render("  ↓ more") + "\n")
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

		badgePlain := string(sess.SourceTool)
		badgeWidth := runewidth.StringWidth(badgePlain)

		// Badge 优先策略：始终预留 Badge 空间
		prefix := "> "
		if !(isSelected && isFocused) {
			prefix = "  "
		}
		prefixWidth := runewidth.StringWidth(prefix)
		spaceWidth := 1 // 标题和 Badge 之间的空格

		// 宽松截断策略：先尝试完整显示标题，超出窗口宽度时才截断
		title := normalizeSingleLine(sess.Title)

		// 用纯文本计算宽度（避免 ANSI 转义序列干扰）
		plainLine := prefix + title + " " + badgePlain
		if runewidth.StringWidth(plainLine) > rightInnerWidth {
			maxTitle := rightInnerWidth - prefixWidth - badgeWidth - spaceWidth
			if maxTitle > 0 {
				title = truncateRunes(title, maxTitle)
			} else {
				title = ""
			}
		}

		// 应用颜色样式到 Badge
		badgeStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7A7A7A")).
			Faint(true)
		switch sess.SourceTool {
		case session.SourceOpenCode:
			badgeStyle = lipgloss.NewStyle().Foreground(openCodeColor).Faint(true)
		case session.SourceClaude:
			badgeStyle = lipgloss.NewStyle().Foreground(claudeColor).Faint(true)
		case session.SourceQwen:
			badgeStyle = lipgloss.NewStyle().Foreground(qwenColor).Faint(true)
		case session.SourceCodex:
			badgeStyle = lipgloss.NewStyle().Foreground(codexColor).Faint(true)
		}
		badge := badgeStyle.Render(badgePlain)

		if isSelected && isFocused {
			// 选中且焦点在右面板：显示高亮
			line := prefix + title + " " + badge
			line = padRightWidth(line, rightInnerWidth)
			rightBody.WriteString(selectedStyle.Render(line) + "\n")
		} else {
			// 未选中或焦点不在右面板
			line := prefix + title + " " + badge
			line = padRightWidth(line, rightInnerWidth)
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
		hiddenEntries := m.hiddenManager.List()
		overlay := m.renderHiddenOverlay(hiddenEntries, width, height)
		view = lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, view) + overlay
	}

	return view
}

func (m model) renderHiddenOverlay(hiddenEntries []hidden.HiddenEntry, width, height int) string {
	overlayWidth := width / 2
	if overlayWidth < 50 {
		overlayWidth = 50
	}

	title := " Hidden Sessions "
	content := fmt.Sprintf("Hidden (%d):\n\n", len(hiddenEntries))

	if len(hiddenEntries) == 0 {
		content += "  (no hidden sessions)"
	} else {
		for i, entry := range hiddenEntries {
			cursor := "  "
			if i == m.hiddenCursor {
				cursor = "> "
			}
			// Truncate title to 35 chars, show first 8 chars of ID
			titleText := truncateRunes(entry.Title, 35)
			idShort := entry.ID
			if len(idShort) > 8 {
				idShort = idShort[:8]
			}
			content += fmt.Sprintf("%s%-35s %s\n", cursor, titleText, idShort)
		}
	}

	// Add operation hints
	content += "\n[r] restore selected  [a] restore all  [esc] close"

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
		{"5", "codex", "Codex", codexColor},
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
		{"Codex", session.NewCodexScanner()},
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

func commandFromTemplate(template string) string {
	expanded := strings.ReplaceAll(template, "{id}", "x")
	expanded = strings.ReplaceAll(expanded, "{project}", "x")
	fields := strings.Fields(expanded)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func commandAvailability(command string) string {
	if command == "" {
		return "missing"
	}
	if path, err := commandBinary(command); err == nil {
		return "ok (" + path + ")"
	}
	return "missing"
}

func windowsSystemBinary(name string) (string, error) {
	path := filepath.Join("/mnt/c/Windows/System32", name)
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return path, nil
	}
	return "", fmt.Errorf("%s not found in /mnt/c/Windows/System32", name)
}

func runDoctor() int {
	cfg := launcherCfg
	inWSL := os.Getenv("WSL_DISTRO_NAME") != ""

	fmt.Println("Agent Session Manager Doctor")
	fmt.Println("============================")
	fmt.Printf("Environment: WSL=%t\n", inWSL)
	fmt.Println()
	fmt.Println("Active launcher config:")
	fmt.Printf("  ASM_TERMINAL_CMD=%s\n", cfg.TerminalCmd)
	fmt.Printf("  ASM_WSL_ENTRY_CMD=%s\n", cfg.WSLEntryCmd)
	fmt.Printf("  ASM_WSL_SHELL=%s\n", cfg.WSLShell)
	fmt.Printf("  ASM_RESTORE_CMD_CLAUDE=%s\n", cfg.RestoreCmdClaude)
	fmt.Printf("  ASM_RESTORE_CMD_OPENCODE=%s\n", cfg.RestoreCmdOpenCode)
	fmt.Printf("  ASM_RESTORE_CMD_QWEN=%s\n", cfg.RestoreCmdQwen)
	fmt.Printf("  ASM_RESTORE_CMD_CODEX=%s\n", cfg.RestoreCmdCodex)
	fmt.Println()
	fmt.Println("Checks:")

	terminalStatus := "missing"
	if bin, err := windowsTerminalBinary(cfg); err == nil {
		terminalStatus = "ok (" + bin + ")"
	}
	fmt.Printf("  terminal binary: %s\n", terminalStatus)

	if inWSL {
		interopStatus := "ok"
		if err := ensureWindowsInteropAvailable(); err != nil {
			interopStatus = "error (" + err.Error() + ")"
		}
		fmt.Printf("  windows interop: %s\n", interopStatus)
	}

	wslStatus := commandAvailability(cfg.WSLEntryCmd)
	if strings.EqualFold(cfg.WSLEntryCmd, "wsl.exe") && wslStatus == "missing" {
		if path, err := windowsSystemBinary("wsl.exe"); err == nil {
			wslStatus = "ok (" + path + ")"
		}
	}
	fmt.Printf("  wsl entry command: %s\n", wslStatus)
	if shellArgs, err := wslShellArgs(cfg); err != nil {
		fmt.Printf("  wsl shell command: error (%s)\n", err.Error())
	} else {
		fmt.Printf("  wsl shell command: %s\n", commandAvailability(shellArgs[0]))
	}
	fmt.Printf("  claude command: %s\n", commandAvailability(commandFromTemplate(cfg.RestoreCmdClaude)))
	fmt.Printf("  opencode command: %s\n", commandAvailability(commandFromTemplate(cfg.RestoreCmdOpenCode)))
	fmt.Printf("  qwen command: %s\n", commandAvailability(commandFromTemplate(cfg.RestoreCmdQwen)))
	fmt.Printf("  codex command: %s\n", commandAvailability(commandFromTemplate(cfg.RestoreCmdCodex)))
	fmt.Println()
	fmt.Println("Tip: export ASM_* vars in your shell profile to customize launcher behavior.")
	return 0
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--doctor" {
		os.Exit(runDoctor())
	}

	hiddenManager, err := hidden.NewManager("agent-session-manager")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to initialize hidden manager: %v\n", err)
	}

	m := model{
		sessions:            collectSessions(),
		searchActive:        false,
		hiddenManager:       hiddenManager,
		showHiddenOverlay:   false,
		hiddenCursor:        0,
		sourceFilter:        "",
		focusPanel:          focusLeft,
		projectCursor:       0,
		sessionCursor:       0,
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
