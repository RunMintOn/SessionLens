package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"agent-session-manager/session"
)

func TestLoadLauncherConfigFromEnv_Defaults(t *testing.T) {
	t.Setenv("ASM_TERMINAL_CMD", "")
	t.Setenv("ASM_TERMINAL_ARGS", "")
	t.Setenv("ASM_WSL_ENTRY_CMD", "")
	t.Setenv("ASM_WSL_SHELL", "")
	t.Setenv("ASM_HOST_SHELL_CMD", "")
	t.Setenv("ASM_HOST_SHELL_ARGS", "")
	t.Setenv("ASM_RESTORE_CMD_CLAUDE", "")
	t.Setenv("ASM_RESTORE_CMD_OPENCODE", "")
	t.Setenv("ASM_RESTORE_CMD_QWEN", "")
	t.Setenv("ASM_RESTORE_CMD_CODEX", "")

	cfg := loadLauncherConfigFromEnv(defaultLauncherConfig(), defaultConfigSources())
	if cfg.TerminalCmd == "" {
		t.Fatal("expected non-empty terminal cmd")
	}
	if cfg.WSLEntryCmd != "wsl.exe" {
		t.Fatalf("expected default wsl entry cmd wsl.exe, got %q", cfg.WSLEntryCmd)
	}
	if cfg.HostShellCmd == "" {
		t.Fatal("expected non-empty host shell cmd")
	}
}

func TestLoadLauncherConfigFromEnv_Overrides(t *testing.T) {
	t.Setenv("ASM_TERMINAL_CMD", "custom-wt")
	t.Setenv("ASM_TERMINAL_ARGS", "--new-window")
	t.Setenv("ASM_WSL_ENTRY_CMD", "custom-wsl")
	t.Setenv("ASM_WSL_SHELL", "bash -lc")
	t.Setenv("ASM_HOST_SHELL_CMD", "bash")
	t.Setenv("ASM_HOST_SHELL_ARGS", "-lc")
	t.Setenv("ASM_RESTORE_CMD_QWEN", "qwen custom {id}")

	cfg := loadLauncherConfigFromEnv(defaultLauncherConfig(), defaultConfigSources())
	if cfg.TerminalCmd != "custom-wt" {
		t.Fatalf("expected terminal override, got %q", cfg.TerminalCmd)
	}
	if len(cfg.TerminalArgs) != 1 || cfg.TerminalArgs[0] != "--new-window" {
		t.Fatalf("expected terminal args override, got %+v", cfg.TerminalArgs)
	}
	if cfg.WSLEntryCmd != "custom-wsl" {
		t.Fatalf("expected wsl entry override, got %q", cfg.WSLEntryCmd)
	}
	if cfg.WSLShell != "bash -lc" {
		t.Fatalf("expected shell override, got %q", cfg.WSLShell)
	}
	if cfg.HostShellCmd != "bash" {
		t.Fatalf("expected host shell override, got %q", cfg.HostShellCmd)
	}
	if cfg.RestoreCmdQwen != "qwen custom {id}" {
		t.Fatalf("expected qwen restore override, got %q", cfg.RestoreCmdQwen)
	}
}

func TestLoadLauncherConfigFromEnv_ArgsCanBeExplicitlyCleared(t *testing.T) {
	base := defaultLauncherConfig()
	if len(base.TerminalArgs) == 0 {
		base.TerminalArgs = []string{"-e"}
	}
	t.Setenv("ASM_TERMINAL_ARGS", "")
	t.Setenv("ASM_HOST_SHELL_ARGS", "")

	cfg := loadLauncherConfigFromEnv(base, defaultConfigSources())
	if len(cfg.TerminalArgs) != 0 {
		t.Fatalf("expected terminal args to be cleared, got %+v", cfg.TerminalArgs)
	}
	if len(cfg.HostShellArgs) != 0 {
		t.Fatalf("expected host shell args to be cleared, got %+v", cfg.HostShellArgs)
	}
}

func TestResolveRestoreScript_UsesTemplateAndQuotes(t *testing.T) {
	cfg := launcherConfig{
		RestoreCmdQwen: "qwen -r {id}",
	}
	sess := session.Session{
		ID:          "abc'def",
		SourceTool:  session.SourceQwen,
		ProjectPath: "/tmp/project's-dir",
	}

	script, err := resolveRestoreScript(sess, cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(script, "cd '/tmp/project'\\''s-dir' && qwen -r 'abc'\\''def'") {
		t.Fatalf("unexpected script: %s", script)
	}
}

func TestResolveRestoreScript_UnresolvedPlaceholder(t *testing.T) {
	cfg := launcherConfig{
		RestoreCmdQwen: "qwen -r {missing}",
	}
	sess := session.Session{
		ID:          "id",
		SourceTool:  session.SourceQwen,
		ProjectPath: "/tmp/project",
	}

	if _, err := resolveRestoreScript(sess, cfg); err == nil {
		t.Fatal("expected unresolved placeholder error")
	}
}

func TestResolveRestoreScript_AllowsBracesInValues(t *testing.T) {
	cfg := launcherConfig{
		RestoreCmdQwen: "qwen -r {id}",
	}
	sess := session.Session{
		ID:          "id-{abc}",
		SourceTool:  session.SourceQwen,
		ProjectPath: "/tmp/proj-{x}",
	}

	if _, err := resolveRestoreScript(sess, cfg); err != nil {
		t.Fatalf("expected braces in values to be allowed, got %v", err)
	}
}

func TestLoadLauncherConfig_UsesConfigFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	raw := `{
		"terminal_cmd":"my-term",
		"terminal_args":["-e"],
		"host_shell_cmd":"bash",
		"host_shell_args":["-lc"],
		"restore_cmd_qwen":"qwen --resume {id}"
	}`
	if err := os.WriteFile(configPath, []byte(raw), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("ASM_CONFIG_PATH", configPath)
	t.Setenv("ASM_TERMINAL_CMD", "")
	t.Setenv("ASM_RESTORE_CMD_QWEN", "")
	cfg, _, path, warning, loaded := loadLauncherConfig()
	if warning != "" {
		t.Fatalf("expected no warning, got %s", warning)
	}
	if !loaded {
		t.Fatal("expected config file to be loaded")
	}
	if path != configPath {
		t.Fatalf("expected config path %q, got %q", configPath, path)
	}
	if cfg.TerminalCmd != "my-term" {
		t.Fatalf("expected config terminal cmd, got %q", cfg.TerminalCmd)
	}
	if cfg.RestoreCmdQwen != "qwen --resume {id}" {
		t.Fatalf("expected config qwen restore, got %q", cfg.RestoreCmdQwen)
	}
}

func TestLoadLauncherConfig_EnvOverridesFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	raw := `{"terminal_cmd":"my-term"}`
	if err := os.WriteFile(configPath, []byte(raw), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("ASM_CONFIG_PATH", configPath)
	t.Setenv("ASM_TERMINAL_CMD", "env-term")
	cfg, _, _, _, _ := loadLauncherConfig()
	if cfg.TerminalCmd != "env-term" {
		t.Fatalf("expected env to override file, got %q", cfg.TerminalCmd)
	}
}

func TestLoadLauncherConfig_WarnsOnReadError(t *testing.T) {
	dirPath := t.TempDir()
	t.Setenv("ASM_CONFIG_PATH", dirPath)
	cfg, _, path, warning, loaded := loadLauncherConfig()
	if cfg.TerminalCmd == "" {
		t.Fatal("expected config to still load defaults on read error")
	}
	if loaded {
		t.Fatal("did not expect config file to be loaded from a directory path")
	}
	if path != dirPath {
		t.Fatalf("expected config path %q, got %q", dirPath, path)
	}
	if warning == "" {
		t.Fatal("expected warning for config read error")
	}
}

func TestParseCLIArgs_DoctorJSON(t *testing.T) {
	opts, err := parseCLIArgs([]string{"--doctor", "--json"})
	if err != nil {
		t.Fatalf("expected no parse error, got %v", err)
	}
	if !opts.doctor || !opts.doctorJSON {
		t.Fatalf("expected doctor json mode, got %+v", opts)
	}
}

func TestParseCLIArgs_InitWriteForceValidation(t *testing.T) {
	if _, err := parseCLIArgs([]string{"--force"}); err == nil {
		t.Fatal("expected --force validation error")
	}
	if _, err := parseCLIArgs([]string{"--init-config", "--force"}); err == nil {
		t.Fatal("expected --force requires --write error")
	}
}

func TestBuildDoctorReport_HasStableSchema(t *testing.T) {
	launcherCfg = defaultLauncherConfig()
	launcherCfgSources = defaultConfigSources()
	launcherCfgPath = ""
	launcherCfgWarning = ""
	launcherCfgLoaded = false

	report := buildDoctorReport()
	if report.SchemaVersion != "doctor.v1" {
		t.Fatalf("expected doctor.v1, got %q", report.SchemaVersion)
	}
	if _, ok := report.Checks["tools"]; !ok {
		t.Fatal("expected tools checks in doctor report")
	}
}

func TestRenderSearchLine_StableHeight(t *testing.T) {
	width := 120
	cases := []struct {
		name   string
		query  string
		active bool
	}{
		{name: "inactive placeholder", query: "/ to search", active: false},
		{name: "inactive long", query: strings.Repeat("search-", 40), active: false},
		{name: "active short", query: "abc", active: true},
		{name: "active cjk", query: "这是一个非常长的搜索词 mixed with english", active: true},
	}

	expected := 0
	for _, tc := range cases {
		line := renderSearchLine(tc.query, width, tc.active)
		height := lipgloss.Height(line)
		if expected == 0 {
			expected = height
		}
		if height != expected {
			t.Fatalf("%s: expected stable height %d, got %d", tc.name, expected, height)
		}
	}
}

func TestView_StableHeightAcrossProjectScrollStates(t *testing.T) {
	m := model{
		sessions:     buildRenderTestSessions(24),
		width:        100,
		height:       20,
		focusPanel:   focusLeft,
		query:        "",
		searchActive: false,
	}

	positions := []int{0, 1, 8, 16, 23}
	expectedHeight := 0

	for _, pos := range positions {
		m.projectCursor = pos
		m.clampProjectCursor()
		m.sessionCursor = 0

		view := m.View()
		got := lipgloss.Height(view)
		if expectedHeight == 0 {
			expectedHeight = got
		}
		if got != expectedHeight {
			t.Fatalf("cursor %d: expected stable view height %d, got %d", pos, expectedHeight, got)
		}
		if got > m.height {
			t.Fatalf("cursor %d: expected view height <= %d, got %d", pos, m.height, got)
		}
		if !strings.Contains(view, "SEARCH>") {
			t.Fatalf("cursor %d: expected search line to be rendered", pos)
		}
	}
}

func buildRenderTestSessions(projectCount int) []session.Session {
	sessions := make([]session.Session, 0, projectCount)
	base := int64(1700000000)
	for i := 0; i < projectCount; i++ {
		sessions = append(sessions, session.Session{
			ID:          fmt.Sprintf("sid-%02d", i),
			Title:       fmt.Sprintf("Project session %02d", i),
			SourceTool:  session.SourceOpenCode,
			ProjectPath: fmt.Sprintf("/tmp/proj-%02d", i),
			LastUpdated: base - int64(i),
		})
	}
	return sessions
}
