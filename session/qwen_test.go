package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQwenScanner_Scan(t *testing.T) {
	// Create a temp directory structure
	tmpDir := t.TempDir()

	// Create project structure: <tmpDir>/-testproject/chats/
	projDir := filepath.Join(tmpDir, "-testproject")
	chatsDir := filepath.Join(projDir, "chats")
	if err := os.MkdirAll(chatsDir, 0755); err != nil {
		t.Fatalf("failed to create chats dir: %v", err)
	}

	// Create a valid JSONL file
	jsonlContent := `{"type":"user","message":{"parts":[{"text":"Hello, help me with this task"}]},"timestamp":1700000000}
{"type":"assistant","message":{"parts":[{"text":"Sure, I can help!"}]},"timestamp":1700000001}
{"type":"user","message":{"parts":[{"text":"Second question"}]},"timestamp":1700000002}
`
	if err := os.WriteFile(filepath.Join(chatsDir, "session1.jsonl"), []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("failed to write jsonl: %v", err)
	}

	// Create scanner with temp base path
	scanner := &QwenScanner{BasePath: tmpDir}
	sessions, err := scanner.Scan("testproject")
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	if sessions[0].Title != "Hello, help me with this task" {
		t.Errorf("expected title 'Hello, help me with this task', got '%s'", sessions[0].Title)
	}

	if sessions[0].SourceTool != SourceQwen {
		t.Errorf("expected SourceTool 'qwen', got '%s'", sessions[0].SourceTool)
	}

	if sessions[0].LastUpdated != 1700000002 {
		t.Errorf("expected LastUpdated 1700000002, got %d", sessions[0].LastUpdated)
	}
}

func TestQwenScanner_MessagePartsExtraction(t *testing.T) {
	tmpDir := t.TempDir()

	projDir := filepath.Join(tmpDir, "-testproject")
	chatsDir := filepath.Join(projDir, "chats")
	os.MkdirAll(chatsDir, 0755)

	// Test extracting text from message.parts
	jsonlContent := `{"type":"user","message":{"parts":[{"text":"First message in parts"}]},"timestamp":1700000001}
{"type":"assistant","message":{"parts":[{"text":"Response"}]},"timestamp":1700000002}
`
	os.WriteFile(filepath.Join(chatsDir, "test.jsonl"), []byte(jsonlContent), 0644)

	scanner := &QwenScanner{BasePath: tmpDir}
	sessions, _ := scanner.Scan("testproject")

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Title != "First message in parts" {
		t.Errorf("expected 'First message in parts', got '%s'", sessions[0].Title)
	}
}

func TestQwenScanner_CreatedAtFallback(t *testing.T) {
	tmpDir := t.TempDir()

	projDir := filepath.Join(tmpDir, "-testproject")
	chatsDir := filepath.Join(projDir, "chats")
	os.MkdirAll(chatsDir, 0755)

	// Test using createdAt when timestamp is missing
	jsonlContent := `{"type":"user","message":{"parts":[{"text":"Test"}]},"createdAt":1800000000}
{"type":"assistant","message":{"parts":[{"text":"Response"}]},"timestamp":1700000000}
`
	os.WriteFile(filepath.Join(chatsDir, "test.jsonl"), []byte(jsonlContent), 0644)

	scanner := &QwenScanner{BasePath: tmpDir}
	sessions, _ := scanner.Scan("testproject")

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	// Should use createdAt (1800000000) which is greater than timestamp (1700000000)
	if sessions[0].LastUpdated != 1800000000 {
		t.Errorf("expected LastUpdated 1800000000, got %d", sessions[0].LastUpdated)
	}
}

func TestQwenScanner_MalformedLineSkip(t *testing.T) {
	tmpDir := t.TempDir()

	projDir := filepath.Join(tmpDir, "-testproject")
	chatsDir := filepath.Join(projDir, "chats")
	os.MkdirAll(chatsDir, 0755)

	// Test skipping malformed JSON lines
	jsonlContent := `{"type":"user","message":{"parts":[{"text":"Valid"}]},"timestamp":1700000001}
not valid json
{"type":"assistant","message":{"parts":[{"text":"Response"}]},"timestamp":1700000002}
{}` + "\n" + `invalid
`
	os.WriteFile(filepath.Join(chatsDir, "test.jsonl"), []byte(jsonlContent), 0644)

	scanner := &QwenScanner{BasePath: tmpDir}
	sessions, _ := scanner.Scan("testproject")

	// Should still get the valid session
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Title != "Valid" {
		t.Errorf("expected title 'Valid', got '%s'", sessions[0].Title)
	}
}

func TestQwenScanner_NoUserTextFallback(t *testing.T) {
	tmpDir := t.TempDir()

	projDir := filepath.Join(tmpDir, "-testproject")
	chatsDir := filepath.Join(projDir, "chats")
	os.MkdirAll(chatsDir, 0755)

	// Test fallback to file stem when no user message
	jsonlContent := `{"type":"assistant","message":{"parts":[{"text":"I am assistant"}]},"timestamp":1700000001}
`
	os.WriteFile(filepath.Join(chatsDir, "my-fallback-session.jsonl"), []byte(jsonlContent), 0644)

	scanner := &QwenScanner{BasePath: tmpDir}
	sessions, _ := scanner.Scan("testproject")

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	// Should fallback to file stem
	if sessions[0].Title != "my-fallback-session" {
		t.Errorf("expected title 'my-fallback-session', got '%s'", sessions[0].Title)
	}
}

func TestQwenScanner_TitleTruncation(t *testing.T) {
	tmpDir := t.TempDir()

	projDir := filepath.Join(tmpDir, "-testproject")
	chatsDir := filepath.Join(projDir, "chats")
	os.MkdirAll(chatsDir, 0755)

	longText := strings.Repeat("q", NormalizedTitleWidth+50)
	jsonlContent := `{"type":"user","message":{"parts":[{"text":"` + longText + `"}]},"timestamp":1700000001}
`
	os.WriteFile(filepath.Join(chatsDir, "test.jsonl"), []byte(jsonlContent), 0644)

	scanner := &QwenScanner{BasePath: tmpDir}
	sessions, _ := scanner.Scan("testproject")

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if len([]rune(sessions[0].Title)) > NormalizedTitleWidth {
		t.Errorf("expected title rune length <= %d, got %d", NormalizedTitleWidth, len([]rune(sessions[0].Title)))
	}
	if !strings.HasSuffix(sessions[0].Title, "…") {
		t.Errorf("expected title to end with ellipsis, got '%s'", sessions[0].Title)
	}
}

func TestQwenScanner_UsesCwdAsProjectPath(t *testing.T) {
	tmpDir := t.TempDir()

	projDir := filepath.Join(tmpDir, "-testproject")
	chatsDir := filepath.Join(projDir, "chats")
	os.MkdirAll(chatsDir, 0755)

	jsonlContent := `{"type":"user","cwd":"/home/lee/11MyProjrct/projectA","message":{"parts":[{"text":"hello"}]},"timestamp":"2026-02-18T17:16:42.012Z"}
`
	os.WriteFile(filepath.Join(chatsDir, "test.jsonl"), []byte(jsonlContent), 0644)

	scanner := &QwenScanner{BasePath: tmpDir}
	sessions, err := scanner.Scan("testproject")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ProjectPath != "/home/lee/11MyProjrct/projectA" {
		t.Fatalf("expected cwd project path, got '%s'", sessions[0].ProjectPath)
	}
}

func TestQwenScanner_EmptyBasePath(t *testing.T) {
	// Test with non-existent base path - should return empty
	scanner := &QwenScanner{BasePath: "/nonexistent/path"}
	sessions, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestNewQwenScanner(t *testing.T) {
	scanner := NewQwenScanner()
	if scanner == nil {
		t.Fatal("expected non-nil scanner")
	}
	if scanner.BasePath == "" {
		t.Error("expected BasePath to be set")
	}
}
