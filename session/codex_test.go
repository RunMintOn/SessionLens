package session

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCodexScanner_ScanBasic(t *testing.T) {
	root := t.TempDir()
	basePath := filepath.Join(root, ".codex", "sessions")
	filePath := filepath.Join(basePath, "2026", "02", "21", "session-a.jsonl")
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	content := ""
	content += `{"timestamp":"2026-02-21T08:45:39.784Z","type":"session_meta","payload":{"id":"uuid-a","cwd":"/tmp/project-a"}}` + "\n"
	content += `{"timestamp":"2026-02-21T08:47:39.784Z","type":"event_msg","payload":{"type":"user_message","message":"build codex adapter"}}` + "\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	scanner := &CodexScanner{BasePath: basePath}
	sessions, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	s := sessions[0]
	if s.ID != "uuid-a" {
		t.Fatalf("expected id uuid-a, got %s", s.ID)
	}
	if s.SourceTool != SourceCodex {
		t.Fatalf("expected source codex, got %s", s.SourceTool)
	}
	if s.Title != "build codex adapter" {
		t.Fatalf("unexpected title: %s", s.Title)
	}
	if s.ProjectPath != "/tmp/project-a" {
		t.Fatalf("unexpected project path: %s", s.ProjectPath)
	}
	if s.LastUpdated == 0 {
		t.Fatal("expected non-zero last updated")
	}
}

func TestCodexScanner_UseSessionMetaID(t *testing.T) {
	root := t.TempDir()
	basePath := filepath.Join(root, ".codex", "sessions")
	filePath := filepath.Join(basePath, "2026", "02", "21", "rollout-abc.jsonl")
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	content := ""
	content += `{"timestamp":"2026-02-21T08:45:39.784Z","type":"session_meta","payload":{"id":"meta-uuid","cwd":"/tmp/project"}}` + "\n"
	content += `{"timestamp":"2026-02-21T08:47:39.784Z","type":"event_msg","payload":{"type":"user_message","message":"hello"}}` + "\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	scanner := &CodexScanner{BasePath: basePath}
	sessions, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != "meta-uuid" {
		t.Fatalf("expected meta id, got %s", sessions[0].ID)
	}
}

func TestCodexScanner_TitleStrictFiltering(t *testing.T) {
	root := t.TempDir()
	basePath := filepath.Join(root, ".codex", "sessions")
	filePath := filepath.Join(basePath, "2026", "02", "21", "session-filter.jsonl")
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	content := ""
	content += `{"timestamp":"2026-02-21T08:45:39.784Z","type":"session_meta","payload":{"id":"uuid-filter","cwd":"/tmp/project"}}` + "\n"
	content += `{"timestamp":"2026-02-21T08:46:39.784Z","type":"response_item","payload":{"role":"user","content":[{"type":"input_text","text":"# AGENTS.md instructions..."}]}}` + "\n"
	content += `{"timestamp":"2026-02-21T08:47:39.784Z","type":"event_msg","payload":{"type":"user_message","message":"<environment_context>"}}` + "\n"
	content += `{"timestamp":"2026-02-21T08:48:39.784Z","type":"response_item","payload":{"role":"user","content":[{"type":"input_text","text":"implement codex scanner now"}]}}` + "\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	scanner := &CodexScanner{BasePath: basePath}
	sessions, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Title != "implement codex scanner now" {
		t.Fatalf("unexpected title: %s", sessions[0].Title)
	}
}

func TestCodexScanner_SkipNoValidTitle(t *testing.T) {
	root := t.TempDir()
	basePath := filepath.Join(root, ".codex", "sessions")
	filePath := filepath.Join(basePath, "2026", "02", "21", "session-empty.jsonl")
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	content := ""
	content += `{"timestamp":"2026-02-21T08:45:39.784Z","type":"session_meta","payload":{"id":"uuid-empty","cwd":"/tmp/project"}}` + "\n"
	content += `{"timestamp":"2026-02-21T08:46:39.784Z","type":"response_item","payload":{"role":"user","content":[{"type":"input_text","text":"# AGENTS.md"}]}}` + "\n"
	content += `{"timestamp":"2026-02-21T08:47:39.784Z","type":"event_msg","payload":{"type":"user_message","message":"<system>"}}` + "\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	scanner := &CodexScanner{BasePath: basePath}
	sessions, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestCodexScanner_ProjectPathPriority(t *testing.T) {
	root := t.TempDir()
	basePath := filepath.Join(root, ".codex", "sessions")
	filePath := filepath.Join(basePath, "2026", "02", "21", "session-cwd.jsonl")
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	content := ""
	content += `{"timestamp":"2026-02-21T08:45:39.784Z","type":"session_meta","payload":{"id":"uuid-cwd","cwd":"/from/meta"}}` + "\n"
	content += `{"timestamp":"2026-02-21T08:47:39.784Z","type":"event_msg","payload":{"type":"user_message","message":"message","cwd":"/from/payload"}}` + "\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	scanner := &CodexScanner{BasePath: basePath}
	sessions, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ProjectPath != "/from/meta" {
		t.Fatalf("expected /from/meta, got %s", sessions[0].ProjectPath)
	}
}

func TestCodexScanner_TimestampFallbackToMtime(t *testing.T) {
	root := t.TempDir()
	basePath := filepath.Join(root, ".codex", "sessions")
	filePath := filepath.Join(basePath, "2026", "02", "21", "session-mtime.jsonl")
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	content := ""
	content += `{"type":"session_meta","payload":{"id":"uuid-mtime","cwd":"/tmp/project"}}` + "\n"
	content += `{"type":"event_msg","payload":{"type":"user_message","message":"fallback mtime"}}` + "\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	expected := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(filePath, expected, expected); err != nil {
		t.Fatalf("failed to set mtime: %v", err)
	}

	scanner := &CodexScanner{BasePath: basePath}
	sessions, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].LastUpdated != expected.Unix() {
		t.Fatalf("expected mtime %d, got %d", expected.Unix(), sessions[0].LastUpdated)
	}
}

func TestCodexScanner_SkipSubagents(t *testing.T) {
	root := t.TempDir()
	basePath := filepath.Join(root, ".codex", "sessions")
	valid := filepath.Join(basePath, "2026", "02", "21", "session-ok.jsonl")
	skipped := filepath.Join(basePath, "2026", "02", "21", "subagents", "session-sub.jsonl")

	if err := os.MkdirAll(filepath.Dir(valid), 0755); err != nil {
		t.Fatalf("failed to create valid dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(skipped), 0755); err != nil {
		t.Fatalf("failed to create skipped dir: %v", err)
	}

	validContent := ""
	validContent += `{"timestamp":"2026-02-21T08:45:39.784Z","type":"session_meta","payload":{"id":"uuid-ok","cwd":"/tmp/project"}}` + "\n"
	validContent += `{"timestamp":"2026-02-21T08:46:39.784Z","type":"event_msg","payload":{"type":"user_message","message":"keep me"}}` + "\n"
	if err := os.WriteFile(valid, []byte(validContent), 0644); err != nil {
		t.Fatalf("failed to write valid file: %v", err)
	}

	subContent := ""
	subContent += `{"timestamp":"2026-02-21T08:45:39.784Z","type":"session_meta","payload":{"id":"uuid-sub","cwd":"/tmp/project"}}` + "\n"
	subContent += `{"timestamp":"2026-02-21T08:46:39.784Z","type":"event_msg","payload":{"type":"user_message","message":"skip me"}}` + "\n"
	if err := os.WriteFile(skipped, []byte(subContent), 0644); err != nil {
		t.Fatalf("failed to write subagent file: %v", err)
	}

	scanner := &CodexScanner{BasePath: basePath}
	sessions, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != "uuid-ok" {
		t.Fatalf("unexpected session id: %s", sessions[0].ID)
	}
}

func TestCodexScanner_FilterByProjectPath(t *testing.T) {
	root := t.TempDir()
	basePath := filepath.Join(root, ".codex", "sessions")
	if err := os.MkdirAll(filepath.Join(basePath, "2026", "02", "21"), 0755); err != nil {
		t.Fatalf("failed to create base dir: %v", err)
	}

	for i, proj := range []string{"/p/a", "/p/b"} {
		filePath := filepath.Join(basePath, "2026", "02", "21", fmt.Sprintf("session-%d.jsonl", i))
		content := ""
		content += fmt.Sprintf(`{"timestamp":"2026-02-21T08:45:39.784Z","type":"session_meta","payload":{"id":"uuid-%d","cwd":"%s"}}`, i, proj) + "\n"
		content += `{"timestamp":"2026-02-21T08:46:39.784Z","type":"event_msg","payload":{"type":"user_message","message":"message"}}` + "\n"
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
	}

	scanner := &CodexScanner{BasePath: basePath}
	sessions, err := scanner.Scan("/p/a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ProjectPath != "/p/a" {
		t.Fatalf("expected /p/a, got %s", sessions[0].ProjectPath)
	}
}

func TestNewCodexScanner_DefaultPath(t *testing.T) {
	scanner := NewCodexScanner()
	if scanner == nil {
		t.Fatal("expected non-nil scanner")
	}
	if scanner.BasePath == "" {
		t.Fatal("expected non-empty base path")
	}
}

func TestCodexScanner_NonexistentPath(t *testing.T) {
	scanner := &CodexScanner{BasePath: "/nonexistent/path/for/codex/sessions"}
	sessions, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}
