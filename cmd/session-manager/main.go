package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
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
	TerminalArgs       []string
	WSLEntryCmd        string
	WSLShell           string
	HostShellCmd       string
	HostShellArgs      []string
	RestoreCmdClaude   string
	RestoreCmdOpenCode string
	RestoreCmdQwen     string
	RestoreCmdCodex    string
}

type launcherConfigFile struct {
	TerminalCmd        string   `json:"terminal_cmd"`
	TerminalArgs       []string `json:"terminal_args"`
	WSLEntryCmd        string   `json:"wsl_entry_cmd"`
	WSLShell           string   `json:"wsl_shell"`
	HostShellCmd       string   `json:"host_shell_cmd"`
	HostShellArgs      []string `json:"host_shell_args"`
	RestoreCmdClaude   string   `json:"restore_cmd_claude"`
	RestoreCmdOpenCode string   `json:"restore_cmd_opencode"`
	RestoreCmdQwen     string   `json:"restore_cmd_qwen"`
	RestoreCmdCodex    string   `json:"restore_cmd_codex"`
}

type cliOptions struct {
	doctor               bool
	doctorJSON           bool
	printEffectiveConfig bool
	initConfig           bool
	write                bool
	force                bool
}

type doctorCheck struct {
	Status  string `json:"status"`
	Details string `json:"details,omitempty"`
}

type doctorSuggestion struct {
	Type        string `json:"type"`
	Title       string `json:"title"`
	Key         string `json:"key,omitempty"`
	Value       string `json:"value,omitempty"`
	Path        string `json:"path,omitempty"`
	JSONPointer string `json:"json_pointer,omitempty"`
	Message     string `json:"message,omitempty"`
}

type doctorReport struct {
	SchemaVersion string             `json:"schema_version"`
	Platform      map[string]any     `json:"platform"`
	Config        map[string]any     `json:"config"`
	Checks        map[string]any     `json:"checks"`
	Suggestions   []doctorSuggestion `json:"suggestions"`
}

var (
	launcherCfg        launcherConfig
	launcherCfgSources map[string]string
	launcherCfgPath    string
	launcherCfgWarning string
	launcherCfgLoaded  bool
)

var placeholderPattern = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

func envOrDefault(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func defaultLauncherConfig() launcherConfig {
	cfg := launcherConfig{
		TerminalCmd:        "x-terminal-emulator",
		TerminalArgs:       []string{"-e"},
		WSLEntryCmd:        "wsl.exe",
		WSLShell:           "zsh -lic",
		HostShellCmd:       "sh",
		HostShellArgs:      []string{"-lc"},
		RestoreCmdClaude:   "claude -r {id}",
		RestoreCmdOpenCode: "opencode -s {id}",
		RestoreCmdQwen:     "qwen -r {id}",
		RestoreCmdCodex:    "codex resume {id}",
	}

	if runtime.GOOS == "windows" || os.Getenv("WSL_DISTRO_NAME") != "" {
		cfg.TerminalCmd = "wt"
		cfg.TerminalArgs = nil
	}
	if runtime.GOOS == "darwin" {
		cfg.TerminalCmd = "open"
		cfg.TerminalArgs = []string{"-a", "Terminal"}
		cfg.HostShellCmd = "zsh"
		cfg.HostShellArgs = []string{"-lc"}
	}
	return cfg
}

func launcherConfigPath() string {
	if custom := strings.TrimSpace(os.Getenv("ASM_CONFIG_PATH")); custom != "" {
		return custom
	}
	configDir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(configDir) == "" {
		return ""
	}
	return filepath.Join(configDir, "agent-session-manager", "config.json")
}

func defaultConfigSources() map[string]string {
	return map[string]string{
		"terminal_cmd":         "default",
		"terminal_args":        "default",
		"wsl_entry_cmd":        "default",
		"wsl_shell":            "default",
		"host_shell_cmd":       "default",
		"host_shell_args":      "default",
		"restore_cmd_claude":   "default",
		"restore_cmd_opencode": "default",
		"restore_cmd_qwen":     "default",
		"restore_cmd_codex":    "default",
	}
}

func applyLauncherFileConfig(cfg *launcherConfig, fileCfg launcherConfigFile, sources map[string]string) {
	if strings.TrimSpace(fileCfg.TerminalCmd) != "" {
		cfg.TerminalCmd = strings.TrimSpace(fileCfg.TerminalCmd)
		sources["terminal_cmd"] = "file"
	}
	if strings.TrimSpace(fileCfg.WSLEntryCmd) != "" {
		cfg.WSLEntryCmd = strings.TrimSpace(fileCfg.WSLEntryCmd)
		sources["wsl_entry_cmd"] = "file"
	}
	if strings.TrimSpace(fileCfg.WSLShell) != "" {
		cfg.WSLShell = strings.TrimSpace(fileCfg.WSLShell)
		sources["wsl_shell"] = "file"
	}
	if strings.TrimSpace(fileCfg.HostShellCmd) != "" {
		cfg.HostShellCmd = strings.TrimSpace(fileCfg.HostShellCmd)
		sources["host_shell_cmd"] = "file"
	}
	if fileCfg.TerminalArgs != nil {
		cfg.TerminalArgs = append([]string{}, fileCfg.TerminalArgs...)
		sources["terminal_args"] = "file"
	}
	if fileCfg.HostShellArgs != nil {
		cfg.HostShellArgs = append([]string{}, fileCfg.HostShellArgs...)
		sources["host_shell_args"] = "file"
	}
	if strings.TrimSpace(fileCfg.RestoreCmdClaude) != "" {
		cfg.RestoreCmdClaude = strings.TrimSpace(fileCfg.RestoreCmdClaude)
		sources["restore_cmd_claude"] = "file"
	}
	if strings.TrimSpace(fileCfg.RestoreCmdOpenCode) != "" {
		cfg.RestoreCmdOpenCode = strings.TrimSpace(fileCfg.RestoreCmdOpenCode)
		sources["restore_cmd_opencode"] = "file"
	}
	if strings.TrimSpace(fileCfg.RestoreCmdQwen) != "" {
		cfg.RestoreCmdQwen = strings.TrimSpace(fileCfg.RestoreCmdQwen)
		sources["restore_cmd_qwen"] = "file"
	}
	if strings.TrimSpace(fileCfg.RestoreCmdCodex) != "" {
		cfg.RestoreCmdCodex = strings.TrimSpace(fileCfg.RestoreCmdCodex)
		sources["restore_cmd_codex"] = "file"
	}
}

func envArgs(key string, fallback []string) []string {
	raw, exists := os.LookupEnv(key)
	if !exists {
		return append([]string{}, fallback...)
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []string{}
	}
	return strings.Fields(trimmed)
}

func loadLauncherConfigFromEnv(base launcherConfig, sources map[string]string) launcherConfig {
	cfg := base
	if _, ok := os.LookupEnv("ASM_TERMINAL_CMD"); ok {
		sources["terminal_cmd"] = "env"
	}
	cfg.TerminalCmd = envOrDefault("ASM_TERMINAL_CMD", cfg.TerminalCmd)
	if _, ok := os.LookupEnv("ASM_TERMINAL_ARGS"); ok {
		sources["terminal_args"] = "env"
	}
	cfg.TerminalArgs = envArgs("ASM_TERMINAL_ARGS", cfg.TerminalArgs)
	if _, ok := os.LookupEnv("ASM_WSL_ENTRY_CMD"); ok {
		sources["wsl_entry_cmd"] = "env"
	}
	cfg.WSLEntryCmd = envOrDefault("ASM_WSL_ENTRY_CMD", cfg.WSLEntryCmd)
	if _, ok := os.LookupEnv("ASM_WSL_SHELL"); ok {
		sources["wsl_shell"] = "env"
	}
	cfg.WSLShell = envOrDefault("ASM_WSL_SHELL", cfg.WSLShell)
	if _, ok := os.LookupEnv("ASM_HOST_SHELL_CMD"); ok {
		sources["host_shell_cmd"] = "env"
	}
	cfg.HostShellCmd = envOrDefault("ASM_HOST_SHELL_CMD", cfg.HostShellCmd)
	if _, ok := os.LookupEnv("ASM_HOST_SHELL_ARGS"); ok {
		sources["host_shell_args"] = "env"
	}
	cfg.HostShellArgs = envArgs("ASM_HOST_SHELL_ARGS", cfg.HostShellArgs)
	if _, ok := os.LookupEnv("ASM_RESTORE_CMD_CLAUDE"); ok {
		sources["restore_cmd_claude"] = "env"
	}
	cfg.RestoreCmdClaude = envOrDefault("ASM_RESTORE_CMD_CLAUDE", cfg.RestoreCmdClaude)
	if _, ok := os.LookupEnv("ASM_RESTORE_CMD_OPENCODE"); ok {
		sources["restore_cmd_opencode"] = "env"
	}
	cfg.RestoreCmdOpenCode = envOrDefault("ASM_RESTORE_CMD_OPENCODE", cfg.RestoreCmdOpenCode)
	if _, ok := os.LookupEnv("ASM_RESTORE_CMD_QWEN"); ok {
		sources["restore_cmd_qwen"] = "env"
	}
	cfg.RestoreCmdQwen = envOrDefault("ASM_RESTORE_CMD_QWEN", cfg.RestoreCmdQwen)
	if _, ok := os.LookupEnv("ASM_RESTORE_CMD_CODEX"); ok {
		sources["restore_cmd_codex"] = "env"
	}
	cfg.RestoreCmdCodex = envOrDefault("ASM_RESTORE_CMD_CODEX", cfg.RestoreCmdCodex)
	return cfg
}

func loadLauncherConfig() (launcherConfig, map[string]string, string, string, bool) {
	cfg := defaultLauncherConfig()
	sources := defaultConfigSources()
	path := launcherConfigPath()
	warning := ""
	loaded := false
	if path != "" {
		raw, err := os.ReadFile(path)
		if err == nil {
			var fileCfg launcherConfigFile
			if unmarshalErr := json.Unmarshal(raw, &fileCfg); unmarshalErr != nil {
				return loadLauncherConfigFromEnv(cfg, sources), sources, path, fmt.Sprintf("invalid launcher config JSON at %s: %v", path, unmarshalErr), false
			}
			applyLauncherFileConfig(&cfg, fileCfg, sources)
			loaded = true
		} else if !os.IsNotExist(err) {
			warning = fmt.Sprintf("failed to read launcher config at %s: %v", path, err)
		}
	}

	cfg = loadLauncherConfigFromEnv(cfg, sources)
	return cfg, sources, path, warning, loaded
}

var (
	openCodeColor = lipgloss.Color("#86EFAC")
	claudeColor   = lipgloss.Color("#FDBA74")
	qwenColor     = lipgloss.Color("#93C5FD")
	codexColor    = lipgloss.Color("#FCA5A5")

	borderColor         = lipgloss.Color("#3C3C3C")
	textColor           = lipgloss.Color("#FAFAFA")
	selectedColor       = lipgloss.Color("#569CD6")
	leftMutedColor      = lipgloss.Color("#7C8A9A")
	leftCardBorderColor = lipgloss.Color("#2F3944")
	rightMutedColor     = lipgloss.Color("#7C8A9A")
	pathMutedColor      = lipgloss.Color("#5B6572")
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(borderColor)

	itemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	projectStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(openCodeColor)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)

	panelStyle = lipgloss.NewStyle().
			Padding(1, 1)

	footerPanelStyle = lipgloss.NewStyle().
				Padding(0, 1)

	searchPrefixStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(selectedColor)

	searchActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Padding(0, 1).
				Border(lipgloss.NormalBorder()).
				BorderForeground(selectedColor)

	searchInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#CBD5E1")).
				Padding(0, 1).
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("#475569"))

	filterActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Border(lipgloss.NormalBorder(), false, false, true, false).
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
	sessionCardRowHeight = 2 // compact list with slight breathing room
	panelVerticalGaps    = 2 // header->panels and panels->footer separators
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
	return os.Getenv("WSL_DISTRO_NAME") != ""
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

	terminalLower := strings.ToLower(strings.TrimSpace(cfg.TerminalCmd))
	if terminalLower != "wt" && terminalLower != "wt.exe" {
		return "", fmt.Errorf("%s not found", cfg.TerminalCmd)
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
	for _, match := range placeholderPattern.FindAllStringSubmatch(template, -1) {
		if len(match) != 2 {
			continue
		}
		name := match[1]
		if name != "id" && name != "project" {
			return "", fmt.Errorf("invalid restore command template: unresolved placeholder")
		}
	}
	replaceValues := map[string]string{
		"id":      shellQuoteSingle(sess.ID),
		"project": shellQuoteSingle(projectPath),
	}
	restoreCmd := renderTemplate(template, replaceValues)
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
		args := append([]string{}, launcherCfg.TerminalArgs...)
		args = append(args, launcherCfg.WSLEntryCmd, "-e")
		args = append(args, wslShell...)
		args = append(args, script)
		wtCmd = exec.Command(wtBin, args...)
	} else {
		args := append([]string{}, launcherCfg.TerminalArgs...)
		args = append(args, launcherCfg.HostShellCmd)
		args = append(args, launcherCfg.HostShellArgs...)
		args = append(args, script)
		wtCmd = exec.Command(wtBin, args...)
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
		titlePrefix = "▍ "
	} else if isSelected {
		titlePrefix = "│ "
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
		BorderForeground(leftCardBorderColor)

	if isSelected && isFocused {
		base = base.BorderForeground(selectedColor).Bold(true)
	} else if isSelected {
		base = base.BorderForeground(openCodeColor)
	}

	return base.Render(content)
}

func formatSessionTimestamp(ts int64) string {
	if ts <= 0 {
		return "unknown"
	}
	eventTime := time.Unix(ts, 0)
	if eventTime.After(time.Now()) {
		return "just now"
	}

	ago := time.Since(eventTime)
	switch {
	case ago < time.Minute:
		return "just now"
	case ago < time.Hour:
		return fmt.Sprintf("%dm ago", int(ago.Minutes()))
	case ago < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(ago.Hours()))
	case ago < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(ago.Hours()/24))
	case ago < 30*24*time.Hour:
		return fmt.Sprintf("%dw ago", int(ago.Hours()/(24*7)))
	default:
		return eventTime.Local().Format("2006-01-02")
	}
}

func sessionBadgeStyle(source session.SourceType) lipgloss.Style {
	switch source {
	case session.SourceOpenCode:
		return lipgloss.NewStyle().Foreground(openCodeColor).Faint(true)
	case session.SourceClaude:
		return lipgloss.NewStyle().Foreground(claudeColor).Faint(true)
	case session.SourceQwen:
		return lipgloss.NewStyle().Foreground(qwenColor).Faint(true)
	case session.SourceCodex:
		return lipgloss.NewStyle().Foreground(codexColor).Faint(true)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#7A7A7A")).Faint(true)
	}
}

func renderSessionCard(sess session.Session, innerWidth int, isSelected, isFocused bool) string {
	if innerWidth < 10 {
		return ""
	}

	prefix := "  "
	if isSelected && isFocused {
		prefix = "▍ "
	} else if isSelected {
		prefix = "│ "
	}

	badgePlain := string(sess.SourceTool)
	badge := sessionBadgeStyle(sess.SourceTool).Render(badgePlain)
	badgeWidth := runewidth.StringWidth(badgePlain)
	timeText := formatSessionTimestamp(sess.LastUpdated)
	timeWidth := runewidth.StringWidth(timeText)

	plainPrefixWidth := runewidth.StringWidth(prefix)
	// Reserve: prefix + space + badge + 2 spaces + time.
	titleWidth := innerWidth - plainPrefixWidth - badgeWidth - timeWidth - 4
	if titleWidth < 4 {
		titleWidth = 4
	}
	title := truncateRunes(normalizeSingleLine(sess.Title), titleWidth)
	leftPart := prefix + title + " " + badge
	leftPartWidth := plainPrefixWidth + runewidth.StringWidth(title) + 1 + badgeWidth
	gap := innerWidth - leftPartWidth - timeWidth
	if gap < 1 {
		gap = 1
	}

	timeStyled := lipgloss.NewStyle().Foreground(rightMutedColor).Render(timeText)
	line := leftPart + strings.Repeat(" ", gap) + timeStyled
	line = padRightWidth(line, innerWidth)

	if isSelected && isFocused {
		return selectedStyle.Foreground(selectedColor).Render(line)
	}
	if isSelected {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1")).Bold(true).Render(line)
	}
	return itemStyle.Render(line)
}

func renderSearchLine(searchValue string, width int, active bool) string {
	searchWidth := width - 2
	if searchWidth < 10 {
		searchWidth = 10
	}

	searchStyle := searchInactiveStyle
	if active {
		searchStyle = searchActiveStyle
	}

	innerWidth := searchWidth - searchStyle.GetHorizontalFrameSize()
	if innerWidth < 1 {
		innerWidth = 1
	}

	prefixPlain := "SEARCH> "
	prefixWidth := runewidth.StringWidth(prefixPlain)
	valueWidth := innerWidth - prefixWidth
	if valueWidth < 0 {
		valueWidth = 0
	}

	plainValue := truncateRunes(searchValue, valueWidth)
	plainContent := prefixPlain + plainValue
	plainContent = padRightWidth(plainContent, innerWidth)

	// Keep the prefix accent color while still measuring width using plain text.
	renderedPrefix := searchPrefixStyle.Render(prefixPlain)
	return searchStyle.Width(searchWidth).Render(renderedPrefix + plainContent[len(prefixPlain):])
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
	searchLine := renderSearchLine(searchValue, width, m.searchActive)
	headerDivider := lipgloss.NewStyle().
		Foreground(borderColor).
		Faint(true).
		Render(strings.Repeat("─", max(1, width)))

	header := strings.Join([]string{
		titleStyle.Render(" Agent Session Manager "),
		searchLine,
		m.renderFilterRow(),
		headerDivider,
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

	footerWidth := width - 2
	if footerWidth < 10 {
		footerWidth = 10
	}
	footer := footerPanelStyle.Width(footerWidth).Render(
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Render(footerText),
	)

	headerHeight := lipgloss.Height(header)
	footerHeight := lipgloss.Height(footer)
	contentHeight := height - headerHeight - footerHeight - panelVerticalGaps
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Split width into left and right panels
	leftPanelWidth := int(float64(width) * leftPanelRatio)
	if leftPanelWidth < leftPanelMinWidth {
		leftPanelWidth = leftPanelMinWidth
	}
	maxLeftWidth := width - rightPanelMinWidth
	if maxLeftWidth < 20 {
		maxLeftWidth = 20
	}
	if leftPanelWidth > maxLeftWidth {
		leftPanelWidth = maxLeftWidth
	}

	rightPanelWidth := width - leftPanelWidth
	if rightPanelWidth < rightPanelMinWidth {
		rightPanelWidth = rightPanelMinWidth
		leftPanelWidth = width - rightPanelWidth
	}
	if leftPanelWidth < 20 {
		leftPanelWidth = 20
		rightPanelWidth = width - leftPanelWidth
	}
	if rightPanelWidth < 20 {
		rightPanelWidth = 20
		leftPanelWidth = width - rightPanelWidth
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

	leftPanel := panelStyle.
		Width(leftPanelWidth).
		Height(contentHeight).
		MaxHeight(contentHeight).
		Render(leftBody.String())

	// Build right panel (sessions for selected project)
	var rightBody strings.Builder
	var selectedProjectPath string
	if len(grouped) > 0 && m.projectCursor < len(grouped) {
		selectedProjectPath = grouped[m.projectCursor].projectPath
		rightBody.WriteString(
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E2E8F0")).Render("SESSIONS") + "\n",
		)
		rightBody.WriteString(
			lipgloss.NewStyle().Foreground(pathMutedColor).Faint(true).
				Render(truncateRunesNoEllipsis(simplifyPath(selectedProjectPath), rightInnerWidth-2)) + "\n\n",
		)
	} else {
		rightBody.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E2E8F0")).Render("SESSIONS") + "\n")
		rightBody.WriteString(lipgloss.NewStyle().Foreground(rightMutedColor).Render("select a project") + "\n\n")
	}

	sessions := m.getSelectedProjectSessions()
	rightHeaderHeight := 3
	rightVisibleCards := (contentHeight - rightHeaderHeight) / sessionCardRowHeight
	if rightVisibleCards < 1 {
		rightVisibleCards = 1
	}
	rightStart, rightEnd := getVisibleWindow(len(sessions), m.sessionCursor, rightVisibleCards)
	if rightStart > 0 {
		rightBody.WriteString(lipgloss.NewStyle().Foreground(rightMutedColor).Render("  ↑ more") + "\n")
	}

	for i := rightStart; i < rightEnd; i++ {
		sess := sessions[i]
		isSelected := i == m.sessionCursor
		isFocused := m.focusPanel == focusRight
		rightBody.WriteString(renderSessionCard(sess, rightInnerWidth, isSelected, isFocused))
		rightBody.WriteString("\n\n")
	}

	if len(sessions) == 0 {
		if selectedProjectPath != "" {
			rightBody.WriteString(lipgloss.NewStyle().Foreground(rightMutedColor).Render("  (no sessions in this project)") + "\n")
		} else {
			rightBody.WriteString(lipgloss.NewStyle().Foreground(rightMutedColor).Render("  (select a project)") + "\n")
		}
	}
	if rightEnd < len(sessions) {
		rightBody.WriteString(lipgloss.NewStyle().Foreground(rightMutedColor).Render("  ↓ more") + "\n")
	}

	rightPanel := panelStyle.
		Width(rightPanelWidth).
		Height(contentHeight).
		MaxHeight(contentHeight).
		Render(rightBody.String())

	// Join panels horizontally
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	view := header + "\n" + panels + "\n" + footer
	view = lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, view)

	if m.showHiddenOverlay && m.hiddenManager != nil {
		hiddenEntries := m.hiddenManager.List()
		overlay := m.renderHiddenOverlay(hiddenEntries, width, height)
		view = lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, view)
		view += lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay)
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

func configToFile(cfg launcherConfig) launcherConfigFile {
	return launcherConfigFile{
		TerminalCmd:        cfg.TerminalCmd,
		TerminalArgs:       append([]string{}, cfg.TerminalArgs...),
		WSLEntryCmd:        cfg.WSLEntryCmd,
		WSLShell:           cfg.WSLShell,
		HostShellCmd:       cfg.HostShellCmd,
		HostShellArgs:      append([]string{}, cfg.HostShellArgs...),
		RestoreCmdClaude:   cfg.RestoreCmdClaude,
		RestoreCmdOpenCode: cfg.RestoreCmdOpenCode,
		RestoreCmdQwen:     cfg.RestoreCmdQwen,
		RestoreCmdCodex:    cfg.RestoreCmdCodex,
	}
}

func parseCLIArgs(args []string) (cliOptions, error) {
	opts := cliOptions{}
	for _, arg := range args {
		switch arg {
		case "--doctor":
			opts.doctor = true
		case "--json":
			opts.doctorJSON = true
		case "--print-effective-config":
			opts.printEffectiveConfig = true
		case "--init-config":
			opts.initConfig = true
		case "--write":
			opts.write = true
		case "--force":
			opts.force = true
		default:
			return opts, fmt.Errorf("unknown option: %s", arg)
		}
	}

	if opts.doctorJSON && !opts.doctor {
		return opts, fmt.Errorf("--json requires --doctor")
	}
	if opts.write && !opts.initConfig {
		return opts, fmt.Errorf("--write requires --init-config")
	}
	if opts.force && !opts.write {
		return opts, fmt.Errorf("--force requires --write")
	}

	modeCount := 0
	if opts.doctor {
		modeCount++
	}
	if opts.printEffectiveConfig {
		modeCount++
	}
	if opts.initConfig {
		modeCount++
	}
	if modeCount > 1 {
		return opts, fmt.Errorf("choose only one mode: --doctor, --print-effective-config, or --init-config")
	}
	return opts, nil
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

func printJSON(v any) int {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode JSON: %v\n", err)
		return 1
	}
	return 0
}

func runPrintEffectiveConfig() int {
	out := map[string]any{
		"schema_version": "effective-config.v1",
		"path":           launcherCfgPath,
		"loaded":         launcherCfgLoaded,
		"warning":        launcherCfgWarning,
		"sources":        launcherCfgSources,
		"config":         configToFile(launcherCfg),
	}
	return printJSON(out)
}

func runInitConfig(write, force bool) int {
	targetPath := launcherConfigPath()
	if targetPath == "" {
		fmt.Fprintln(os.Stderr, "unable to determine config path; set ASM_CONFIG_PATH")
		return 1
	}

	template := configToFile(defaultLauncherConfig())
	raw, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate config template: %v\n", err)
		return 1
	}

	if !write {
		fmt.Printf("init-config dry-run\npath: %s\n\n%s\n", targetPath, string(raw))
		return 0
	}

	if _, statErr := os.Stat(targetPath); statErr == nil && !force {
		fmt.Fprintf(os.Stderr, "config file already exists: %s (use --force to overwrite)\n", targetPath)
		return 1
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create config directory: %v\n", err)
		return 1
	}
	if err := os.WriteFile(targetPath, append(raw, '\n'), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write config file: %v\n", err)
		return 1
	}
	fmt.Printf("wrote config: %s\n", targetPath)
	return 0
}

func doctorCheckFromCommand(command string) doctorCheck {
	if command == "" {
		return doctorCheck{Status: "missing", Details: "empty command"}
	}
	if path, err := commandBinary(command); err == nil {
		return doctorCheck{Status: "ok", Details: path}
	}
	return doctorCheck{Status: "missing", Details: "command not found"}
}

func buildDoctorReport() doctorReport {
	cfg := launcherCfg
	inWSL := os.Getenv("WSL_DISTRO_NAME") != ""

	terminalCheck := doctorCheck{Status: "missing", Details: "not found"}
	if bin, err := windowsTerminalBinary(cfg); err == nil {
		terminalCheck = doctorCheck{Status: "ok", Details: bin}
	}

	interopCheck := doctorCheck{Status: "ok"}
	if inWSL {
		if err := ensureWindowsInteropAvailable(); err != nil {
			interopCheck = doctorCheck{Status: "error", Details: err.Error()}
		}
	}

	wslEntryCheck := doctorCheckFromCommand(cfg.WSLEntryCmd)
	if strings.EqualFold(cfg.WSLEntryCmd, "wsl.exe") && wslEntryCheck.Status == "missing" {
		if path, err := windowsSystemBinary("wsl.exe"); err == nil {
			wslEntryCheck = doctorCheck{Status: "ok", Details: path}
		}
	}
	wslShellCheck := doctorCheck{Status: "error", Details: "invalid shell args"}
	if shellArgs, err := wslShellArgs(cfg); err != nil {
		wslShellCheck = doctorCheck{Status: "error", Details: err.Error()}
	} else {
		wslShellCheck = doctorCheckFromCommand(shellArgs[0])
	}

	hostShellCheck := doctorCheckFromCommand(cfg.HostShellCmd)
	tools := map[string]doctorCheck{
		"claude":   doctorCheckFromCommand(commandFromTemplate(cfg.RestoreCmdClaude)),
		"opencode": doctorCheckFromCommand(commandFromTemplate(cfg.RestoreCmdOpenCode)),
		"qwen":     doctorCheckFromCommand(commandFromTemplate(cfg.RestoreCmdQwen)),
		"codex":    doctorCheckFromCommand(commandFromTemplate(cfg.RestoreCmdCodex)),
	}

	suggestions := []doctorSuggestion{}
	if terminalCheck.Status != "ok" {
		suggestions = append(suggestions, doctorSuggestion{
			Type:    "set_env",
			Title:   "set terminal command",
			Key:     "ASM_TERMINAL_CMD",
			Value:   cfg.TerminalCmd,
			Message: "set a terminal executable available in PATH",
		})
	}
	if interopCheck.Status == "error" {
		suggestions = append(suggestions, doctorSuggestion{
			Type:    "manual_hint",
			Title:   "verify WSL interop",
			Message: "WSL cannot spawn Windows processes in current session",
		})
	}
	for toolName, check := range tools {
		if check.Status != "ok" {
			key := "ASM_RESTORE_CMD_" + strings.ToUpper(toolName)
			suggestions = append(suggestions, doctorSuggestion{
				Type:    "set_env",
				Title:   "override restore command for " + toolName,
				Key:     key,
				Value:   fmt.Sprintf("%s -r {id}", toolName),
				Message: "set to a command available in your shell",
			})
		}
	}

	return doctorReport{
		SchemaVersion: "doctor.v1",
		Platform: map[string]any{
			"goos":       runtime.GOOS,
			"is_wsl":     inWSL,
			"shell":      os.Getenv("SHELL"),
			"wsl_distro": os.Getenv("WSL_DISTRO_NAME"),
		},
		Config: map[string]any{
			"path":      launcherCfgPath,
			"loaded":    launcherCfgLoaded,
			"warning":   launcherCfgWarning,
			"effective": configToFile(cfg),
			"sources":   launcherCfgSources,
		},
		Checks: map[string]any{
			"terminal_binary": terminalCheck,
			"windows_interop": interopCheck,
			"wsl_entry":       wslEntryCheck,
			"wsl_shell":       wslShellCheck,
			"host_shell":      hostShellCheck,
			"tools":           tools,
		},
		Suggestions: suggestions,
	}
}

func runDoctorText() int {
	report := buildDoctorReport()
	fmt.Println("Agent Session Manager Doctor")
	fmt.Println("============================")
	fmt.Printf("Environment: WSL=%v\n", report.Platform["is_wsl"])
	fmt.Println()
	fmt.Println("Active launcher config:")
	configMap := report.Config
	path, _ := configMap["path"].(string)
	if path != "" {
		fmt.Printf("  config file=%s\n", path)
	} else {
		fmt.Println("  config file=(not found)")
	}
	effective, _ := configMap["effective"].(launcherConfigFile)
	fmt.Printf("  ASM_TERMINAL_CMD=%s\n", effective.TerminalCmd)
	fmt.Printf("  ASM_TERMINAL_ARGS=%s\n", strings.Join(effective.TerminalArgs, " "))
	fmt.Printf("  ASM_WSL_ENTRY_CMD=%s\n", effective.WSLEntryCmd)
	fmt.Printf("  ASM_WSL_SHELL=%s\n", effective.WSLShell)
	fmt.Printf("  ASM_HOST_SHELL_CMD=%s\n", effective.HostShellCmd)
	fmt.Printf("  ASM_HOST_SHELL_ARGS=%s\n", strings.Join(effective.HostShellArgs, " "))
	fmt.Printf("  ASM_RESTORE_CMD_CLAUDE=%s\n", effective.RestoreCmdClaude)
	fmt.Printf("  ASM_RESTORE_CMD_OPENCODE=%s\n", effective.RestoreCmdOpenCode)
	fmt.Printf("  ASM_RESTORE_CMD_QWEN=%s\n", effective.RestoreCmdQwen)
	fmt.Printf("  ASM_RESTORE_CMD_CODEX=%s\n", effective.RestoreCmdCodex)
	fmt.Println()
	fmt.Println("Checks:")
	checks := report.Checks
	printCheck := func(name string, check doctorCheck) {
		if check.Details != "" {
			fmt.Printf("  %s: %s (%s)\n", name, check.Status, check.Details)
		} else {
			fmt.Printf("  %s: %s\n", name, check.Status)
		}
	}
	printCheck("terminal binary", checks["terminal_binary"].(doctorCheck))
	printCheck("windows interop", checks["windows_interop"].(doctorCheck))
	printCheck("wsl entry command", checks["wsl_entry"].(doctorCheck))
	printCheck("wsl shell command", checks["wsl_shell"].(doctorCheck))
	printCheck("host shell command", checks["host_shell"].(doctorCheck))
	tools := checks["tools"].(map[string]doctorCheck)
	printCheck("claude command", tools["claude"])
	printCheck("opencode command", tools["opencode"])
	printCheck("qwen command", tools["qwen"])
	printCheck("codex command", tools["codex"])
	fmt.Println()
	fmt.Println("Tip: export ASM_* vars in your shell profile to customize launcher behavior.")
	return 0
}

func runDoctorJSON() int {
	return printJSON(buildDoctorReport())
}

func initLauncherConfig() {
	cfg, sources, path, warning, loaded := loadLauncherConfig()
	launcherCfg = cfg
	launcherCfgSources = sources
	launcherCfgPath = path
	launcherCfgWarning = warning
	launcherCfgLoaded = loaded
}

func main() {
	opts, parseErr := parseCLIArgs(os.Args[1:])
	if parseErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", parseErr)
		os.Exit(2)
	}

	initLauncherConfig()

	if launcherCfgWarning != "" {
		fmt.Fprintf(os.Stderr, "warning: %s\n", launcherCfgWarning)
	}

	if opts.doctor {
		if opts.doctorJSON {
			os.Exit(runDoctorJSON())
		}
		os.Exit(runDoctorText())
	}
	if opts.printEffectiveConfig {
		os.Exit(runPrintEffectiveConfig())
	}
	if opts.initConfig {
		os.Exit(runInitConfig(opts.write, opts.force))
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
