package main

import (
	"strings"
	"testing"

	"agent-session-manager/session"
)

func TestLoadLauncherConfigFromEnv_Defaults(t *testing.T) {
	t.Setenv("ASM_TERMINAL_CMD", "")
	t.Setenv("ASM_WSL_ENTRY_CMD", "")
	t.Setenv("ASM_WSL_SHELL", "")
	t.Setenv("ASM_RESTORE_CMD_CLAUDE", "")
	t.Setenv("ASM_RESTORE_CMD_OPENCODE", "")
	t.Setenv("ASM_RESTORE_CMD_QWEN", "")
	t.Setenv("ASM_RESTORE_CMD_CODEX", "")

	cfg := loadLauncherConfigFromEnv()
	if cfg.TerminalCmd != "wt" {
		t.Fatalf("expected default terminal cmd wt, got %q", cfg.TerminalCmd)
	}
	if cfg.WSLEntryCmd != "wsl.exe" {
		t.Fatalf("expected default wsl entry cmd wsl.exe, got %q", cfg.WSLEntryCmd)
	}
	if cfg.WSLShell != "zsh -lic" {
		t.Fatalf("expected default wsl shell zsh -lic, got %q", cfg.WSLShell)
	}
}

func TestLoadLauncherConfigFromEnv_Overrides(t *testing.T) {
	t.Setenv("ASM_TERMINAL_CMD", "custom-wt")
	t.Setenv("ASM_WSL_ENTRY_CMD", "custom-wsl")
	t.Setenv("ASM_WSL_SHELL", "bash -lc")
	t.Setenv("ASM_RESTORE_CMD_QWEN", "qwen custom {id}")

	cfg := loadLauncherConfigFromEnv()
	if cfg.TerminalCmd != "custom-wt" {
		t.Fatalf("expected terminal override, got %q", cfg.TerminalCmd)
	}
	if cfg.WSLEntryCmd != "custom-wsl" {
		t.Fatalf("expected wsl entry override, got %q", cfg.WSLEntryCmd)
	}
	if cfg.WSLShell != "bash -lc" {
		t.Fatalf("expected shell override, got %q", cfg.WSLShell)
	}
	if cfg.RestoreCmdQwen != "qwen custom {id}" {
		t.Fatalf("expected qwen restore override, got %q", cfg.RestoreCmdQwen)
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
