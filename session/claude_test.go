package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeScanner_StringContent(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "test-project")
	os.MkdirAll(projDir, 0755)

	jsonlContent := `{"type": "user", "timestamp": 1700000000, "message": {"content": "Hello, how are you?"}}
{"type": "assistant", "timestamp": 1700000001, "message": {"content": "I'm doing well!"}}
`
	jsonlPath := filepath.Join(projDir, "session-001.jsonl")
	os.WriteFile(jsonlPath, []byte(jsonlContent), 0644)

	scanner := &ClaudeScanner{BasePath: tmpDir}
	sessions, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("Expected 1 session, got %d", len(sessions))
	}

	if sessions[0].Title != "Hello, how are you?" {
		t.Errorf("Expected title 'Hello, how are you?', got '%s'", sessions[0].Title)
	}

	if sessions[0].LastUpdated != 1700000001 {
		t.Errorf("Expected last_updated 1700000001, got %d", sessions[0].LastUpdated)
	}

	if sessions[0].SourceTool != SourceClaude {
		t.Errorf("Expected source_tool 'claude', got '%s'", sessions[0].SourceTool)
	}
}

func TestClaudeScanner_ListContent(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "test-project")
	os.MkdirAll(projDir, 0755)

	jsonlContent := `{"type": "user", "timestamp": 1700000000, "message": {"content": [{"type": "text", "text": "This is a test message"}]}}
{"type": "assistant", "timestamp": 1700000001, "message": {"content": "Response"}}
`
	jsonlPath := filepath.Join(projDir, "session-002.jsonl")
	os.WriteFile(jsonlPath, []byte(jsonlContent), 0644)

	scanner := &ClaudeScanner{BasePath: tmpDir}
	sessions, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("Expected 1 session, got %d", len(sessions))
	}

	if sessions[0].Title != "This is a test message" {
		t.Errorf("Expected title 'This is a test message', got '%s'", sessions[0].Title)
	}
}

func TestClaudeScanner_MalformedJsonLine(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "test-project")
	os.MkdirAll(projDir, 0755)

	jsonlContent := `{"type": "user", "timestamp": 1700000000, "message": {"content": "Valid message"}}
{invalid json here}
{"type": "assistant", "timestamp": 1700000001, "message": {"content": "Another message"}}
`
	jsonlPath := filepath.Join(projDir, "session-003.jsonl")
	os.WriteFile(jsonlPath, []byte(jsonlContent), 0644)

	scanner := &ClaudeScanner{BasePath: tmpDir}
	sessions, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("Expected 1 session (malformed line skipped), got %d", len(sessions))
	}

	if sessions[0].Title != "Valid message" {
		t.Errorf("Expected title 'Valid message', got '%s'", sessions[0].Title)
	}
}

func TestClaudeScanner_FallbackTitle(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "test-project")
	os.MkdirAll(projDir, 0755)

	jsonlContent := `{"type": "assistant", "timestamp": 1700000000, "message": {"content": "Only assistant messages"}}
{"type": "assistant", "timestamp": 1700000001, "message": {"content": "No user messages here"}}
`
	jsonlPath := filepath.Join(projDir, "fallback-session.jsonl")
	os.WriteFile(jsonlPath, []byte(jsonlContent), 0644)

	scanner := &ClaudeScanner{BasePath: tmpDir}
	sessions, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("Expected 1 session, got %d", len(sessions))
	}

	if sessions[0].Title != "fallback-session" {
		t.Errorf("Expected title 'fallback-session', got '%s'", sessions[0].Title)
	}
}

func TestClaudeScanner_LongContentTruncated(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "test-project")
	os.MkdirAll(projDir, 0755)

	longContent := "This is a very long message that should be truncated to exactly one hundred characters when it is processed"
	jsonlContent := `{"type": "user", "timestamp": 1700000000, "message": {"content": "` + longContent + `"}}
`
	jsonlPath := filepath.Join(projDir, "session-004.jsonl")
	os.WriteFile(jsonlPath, []byte(jsonlContent), 0644)

	scanner := &ClaudeScanner{BasePath: tmpDir}
	sessions, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("Expected 1 session, got %d", len(sessions))
	}

	if len(sessions[0].Title) != 103 { // 100 chars + "..."
		t.Errorf("Expected title length 103, got %d: '%s'", len(sessions[0].Title), sessions[0].Title)
	}

	if sessions[0].Title != longContent[:100]+"..." {
		t.Errorf("Title not properly truncated")
	}
}

func TestClaudeScanner_NonExistentPath(t *testing.T) {
	scanner := &ClaudeScanner{BasePath: "/nonexistent/path/12345"}
	sessions, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(sessions) != 0 {
		t.Fatalf("Expected 0 sessions, got %d", len(sessions))
	}
}

func TestClaudeScanner_EmptyPath(t *testing.T) {
	scanner := &ClaudeScanner{BasePath: ""}
	sessions, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(sessions) != 0 {
		t.Fatalf("Expected 0 sessions, got %d", len(sessions))
	}
}

func TestNewClaudeScanner(t *testing.T) {
	scanner := NewClaudeScanner()
	if scanner == nil {
		t.Fatal("NewClaudeScanner returned nil")
	}
	if scanner.BasePath == "" {
		t.Error("Expected BasePath to be set")
	}
}

func TestClaudeScanner_ProjectPathFiltering(t *testing.T) {
	tmpDir := t.TempDir()

	projA := filepath.Join(tmpDir, "project-a")
	projB := filepath.Join(tmpDir, "project-b")
	os.MkdirAll(projA, 0755)
	os.MkdirAll(projB, 0755)

	jsonlA := `{"type": "user", "timestamp": 1700000000, "message": {"content": "Project A session"}}
`
	jsonlB := `{"type": "user", "timestamp": 1700000001, "message": {"content": "Project B session"}}
`
	os.WriteFile(filepath.Join(projA, "session-a.jsonl"), []byte(jsonlA), 0644)
	os.WriteFile(filepath.Join(projB, "session-b.jsonl"), []byte(jsonlB), 0644)

	scanner := &ClaudeScanner{BasePath: tmpDir}

	sessionsA, err := scanner.Scan("project-a")
	if err != nil {
		t.Fatalf("Scan(project-a) failed: %v", err)
	}
	if len(sessionsA) != 1 {
		t.Fatalf("Expected 1 session from project-a, got %d", len(sessionsA))
	}
	if sessionsA[0].Title != "Project A session" {
		t.Errorf("Expected 'Project A session', got '%s'", sessionsA[0].Title)
	}

	sessionsB, err := scanner.Scan("project-b")
	if err != nil {
		t.Fatalf("Scan(project-b) failed: %v", err)
	}
	if len(sessionsB) != 1 {
		t.Fatalf("Expected 1 session from project-b, got %d", len(sessionsB))
	}
	if sessionsB[0].Title != "Project B session" {
		t.Errorf("Expected 'Project B session', got '%s'", sessionsB[0].Title)
	}

	sessionsAll, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Scan(\"\") failed: %v", err)
	}
	if len(sessionsAll) != 2 {
		t.Fatalf("Expected 2 sessions total, got %d", len(sessionsAll))
	}
}
